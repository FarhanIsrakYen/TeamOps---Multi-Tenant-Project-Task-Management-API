package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	authmodel "github.com/example/teamops/backend/internal/modules/auth/model"
	usermodel "github.com/example/teamops/backend/internal/modules/users/model"
	sharedauth "github.com/example/teamops/backend/internal/shared/auth"
	"github.com/example/teamops/backend/internal/shared/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

type UserRepository interface {
	Create(context.Context, string, string, string) (usermodel.User, error)
	ByEmail(context.Context, string) (usermodel.User, error)
	ByID(context.Context, uuid.UUID) (usermodel.User, error)
}
type SessionRepository interface {
	CreateSession(context.Context, authmodel.Session, string, string) error
	RotateSession(context.Context, []byte, authmodel.Session, string, string) (authmodel.Session, error)
	RevokeSession(context.Context, []byte) (uuid.UUID, error)
}
type Auditor interface {
	Record(context.Context, uuid.UUID, uuid.UUID, string, string, string, string, map[string]any) error
}
type Transactor interface {
	WithinTransaction(context.Context, func(context.Context) error) error
}
type Result struct {
	AccessToken          string    `json:"accessToken"`
	AccessTokenExpiresAt time.Time `json:"accessTokenExpiresAt"`
	RefreshToken         string    `json:"refreshToken"`
	User                 usermodel.User
}
type Service struct {
	users      UserRepository
	sessions   SessionRepository
	tx         Transactor
	tokens     *sharedauth.TokenManager
	refreshTTL time.Duration
	hashCost   int
	dummyHash  []byte
	audit      Auditor
}

func New(users UserRepository, sessions SessionRepository, tx Transactor, tokens *sharedauth.TokenManager, refreshTTL time.Duration, hashCost int, audit Auditor) *Service {
	dummyHash, _ := bcrypt.GenerateFromPassword([]byte("TeamOps-Dummy-Password-1!"), hashCost)
	return &Service{users: users, sessions: sessions, tx: tx, tokens: tokens, refreshTTL: refreshTTL, hashCost: hashCost, dummyHash: dummyHash, audit: audit}
}
func (s *Service) Register(ctx context.Context, email, name, password, agent, ip string) (Result, error) {
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return Result{}, err
	}
	if err := validatePassword(password); err != nil {
		return Result{}, err
	}
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) < 2 || utf8.RuneCountInString(name) > 100 {
		return Result{}, apperror.New(400, "invalid_name", "name must be between 2 and 100 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.hashCost)
	if err != nil {
		return Result{}, fmt.Errorf("hash password: %w", err)
	}
	var result Result
	err = s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		u, createErr := s.users.Create(txCtx, normalizedEmail, name, string(hash))
		if createErr != nil {
			return createErr
		}
		result, createErr = s.issue(txCtx, u, uuid.New(), agent, ip)
		return createErr
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Result{}, apperror.Wrap(err, 409, "email_taken", "an account with this email already exists")
		}
		return Result{}, fmt.Errorf("create account: %w", err)
	}
	_ = s.record(ctx, uuid.Nil, result.User.ID, "user.registered", "user", result.User.ID.String(), nil)
	return result, nil
}
func (s *Service) Login(ctx context.Context, email, password, agent, ip string) (Result, error) {
	normalizedEmail, normalizeErr := normalizeEmail(email)
	if normalizeErr != nil {
		return Result{}, apperror.New(401, "invalid_credentials", "email or password is incorrect")
	}
	u, err := s.users.ByEmail(ctx, normalizedEmail)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
		return Result{}, apperror.New(401, "invalid_credentials", "email or password is incorrect")
	}
	if err == nil && bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return Result{}, apperror.New(401, "invalid_credentials", "email or password is incorrect")
	}
	if err != nil {
		return Result{}, fmt.Errorf("find user: %w", err)
	}
	result, err := s.issue(ctx, u, uuid.New(), agent, ip)
	if err == nil {
		_ = s.record(ctx, uuid.Nil, u.ID, "auth.login", "user", u.ID.String(), nil)
	}
	return result, err
}
func (s *Service) issue(ctx context.Context, u usermodel.User, family uuid.UUID, agent, ip string) (Result, error) {
	agent, ip = sanitizeSessionMetadata(agent, ip)
	access, exp, err := s.tokens.AccessToken(u.ID, u.Email)
	if err != nil {
		return Result{}, err
	}
	raw, hash, err := sharedauth.NewRefreshToken()
	if err != nil {
		return Result{}, err
	}
	sess := authmodel.Session{ID: uuid.New(), FamilyID: family, UserID: u.ID, TokenHash: hash, ExpiresAt: time.Now().Add(s.refreshTTL)}
	if err = s.sessions.CreateSession(ctx, sess, agent, ip); err != nil {
		return Result{}, fmt.Errorf("create session: %w", err)
	}
	return Result{AccessToken: access, AccessTokenExpiresAt: exp, RefreshToken: raw, User: u}, nil
}
func (s *Service) Refresh(ctx context.Context, raw, agent, ip string) (Result, error) {
	agent, ip = sanitizeSessionMetadata(agent, ip)
	newRaw, newHash, err := sharedauth.NewRefreshToken()
	if err != nil {
		return Result{}, err
	}
	next := authmodel.Session{ID: uuid.New(), TokenHash: newHash, ExpiresAt: time.Now().Add(s.refreshTTL)}
	old, err := s.sessions.RotateSession(ctx, sharedauth.HashRefreshToken(raw), next, agent, ip)
	if err != nil {
		if errors.Is(err, authmodel.ErrRefreshSessionInvalid) {
			return Result{}, apperror.New(401, "invalid_refresh_token", "refresh token is invalid or has been reused")
		}
		return Result{}, fmt.Errorf("rotate refresh session: %w", err)
	}
	u, err := s.users.ByID(ctx, old.UserID)
	if err != nil {
		return Result{}, fmt.Errorf("load session user: %w", err)
	}
	access, exp, err := s.tokens.AccessToken(u.ID, u.Email)
	if err != nil {
		return Result{}, err
	}
	_ = s.record(ctx, uuid.Nil, u.ID, "auth.token_refreshed", "user", u.ID.String(), nil)
	return Result{AccessToken: access, AccessTokenExpiresAt: exp, RefreshToken: newRaw, User: u}, nil
}

func sanitizeSessionMetadata(agent, ip string) (string, string) {
	agent = strings.TrimSpace(agent)
	if len(agent) > 512 {
		agent = agent[:512]
	}
	ip = strings.TrimSpace(ip)
	if len(ip) > 64 {
		ip = ""
	}
	return agent, ip
}
func (s *Service) Logout(ctx context.Context, raw string) error {
	userID, err := s.sessions.RevokeSession(ctx, sharedauth.HashRefreshToken(raw))
	if err != nil {
		return fmt.Errorf("revoke refresh session: %w", err)
	}
	if userID != uuid.Nil {
		_ = s.record(ctx, uuid.Nil, userID, "auth.logout", "user", userID.String(), nil)
	}
	return nil
}

func (s *Service) record(ctx context.Context, orgID, actorID uuid.UUID, action, kind, resourceID string, metadata map[string]any) error {
	if s.audit == nil {
		return nil
	}
	return s.audit.Record(ctx, orgID, actorID, action, kind, resourceID, "", metadata)
}

func normalizeEmail(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" || len(normalized) > 254 {
		return "", apperror.New(400, "invalid_email", "email address is invalid")
	}
	address, err := mail.ParseAddress(normalized)
	if err != nil || address.Address != normalized {
		return "", apperror.New(400, "invalid_email", "email address is invalid")
	}
	return normalized, nil
}

func validatePassword(password string) error {
	if len(password) > 72 {
		return apperror.New(400, "weak_password", "password must be at most 72 bytes")
	}
	if utf8.RuneCountInString(password) < 12 {
		return apperror.New(400, "weak_password", "password must be at least 12 characters")
	}
	var lower, upper, digit, symbol bool
	for _, r := range password {
		lower = lower || unicode.IsLower(r)
		upper = upper || unicode.IsUpper(r)
		digit = digit || unicode.IsDigit(r)
		symbol = symbol || unicode.IsPunct(r) || unicode.IsSymbol(r)
	}
	if !lower || !upper || !digit || !symbol {
		return apperror.New(400, "weak_password", "password must include uppercase, lowercase, number, and symbol characters")
	}
	return nil
}
