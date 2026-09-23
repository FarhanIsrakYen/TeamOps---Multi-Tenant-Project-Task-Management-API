package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	authmodel "github.com/example/teamops/backend/internal/modules/auth/model"
	usermodel "github.com/example/teamops/backend/internal/modules/users/model"
	sharedauth "github.com/example/teamops/backend/internal/shared/auth"
	apperror "github.com/example/teamops/backend/internal/shared/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

const strongPassword = "Correct-Horse-Battery-9!"

type fakeRepository struct {
	user      usermodel.User
	sessions  map[string]*authmodel.Session
	createErr error
}

type fakeTransactor struct{ calls int }

type recordingAuthAuditor struct {
	actions []string
}

func (a *recordingAuthAuditor) Record(_ context.Context, _, _ uuid.UUID, action, _, _, _ string, _ map[string]any) error {
	a.actions = append(a.actions, action)
	return nil
}

func (t *fakeTransactor) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	t.calls++
	return fn(ctx)
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{sessions: make(map[string]*authmodel.Session)}
}

func (f *fakeRepository) Create(_ context.Context, email, name, hash string) (usermodel.User, error) {
	if f.createErr != nil {
		return usermodel.User{}, f.createErr
	}
	f.user = usermodel.User{ID: uuid.New(), Email: email, Name: name, PasswordHash: hash}
	return f.user, nil
}

func (f *fakeRepository) ByEmail(_ context.Context, email string) (usermodel.User, error) {
	if f.user.ID == uuid.Nil || f.user.Email != email {
		return usermodel.User{}, pgx.ErrNoRows
	}
	return f.user, nil
}

func (f *fakeRepository) ByID(_ context.Context, id uuid.UUID) (usermodel.User, error) {
	if f.user.ID == uuid.Nil || f.user.ID != id {
		return usermodel.User{}, pgx.ErrNoRows
	}
	return f.user, nil
}

func (f *fakeRepository) CreateSession(_ context.Context, session authmodel.Session, _, _ string) error {
	copy := session
	f.sessions[string(session.TokenHash)] = &copy
	return nil
}

func (f *fakeRepository) RotateSession(_ context.Context, hash []byte, next authmodel.Session, _, _ string) (authmodel.Session, error) {
	old, ok := f.sessions[string(hash)]
	if !ok {
		return authmodel.Session{}, authmodel.ErrRefreshSessionInvalid
	}
	if old.RevokedAt != nil || time.Now().After(old.ExpiresAt) {
		f.revokeFamily(old.FamilyID)
		return *old, authmodel.ErrRefreshSessionInvalid
	}
	now := time.Now()
	old.RevokedAt = &now
	next.FamilyID = old.FamilyID
	next.UserID = old.UserID
	copy := next
	f.sessions[string(next.TokenHash)] = &copy
	return *old, nil
}

func (f *fakeRepository) RevokeSession(_ context.Context, hash []byte) (uuid.UUID, error) {
	if session, ok := f.sessions[string(hash)]; ok {
		f.revokeFamily(session.FamilyID)
		return session.UserID, nil
	}
	return uuid.Nil, nil
}

func (f *fakeRepository) revokeFamily(familyID uuid.UUID) {
	now := time.Now()
	for _, session := range f.sessions {
		if session.FamilyID == familyID && session.RevokedAt == nil {
			session.RevokedAt = &now
		}
	}
}

func newService(repo *fakeRepository) *Service {
	tokens := sharedauth.NewTokenManager("this-is-a-long-enough-test-secret-value", "teamops-api", "teamops-web", time.Minute)
	return New(repo, repo, &fakeTransactor{}, tokens, time.Hour, bcrypt.MinCost, nil)
}

func TestRegisterNormalizesEmailHashesPasswordAndCreatesSession(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	result, err := newService(repo).Register(context.Background(), "  Person@Example.COM ", " Person ", strongPassword, "agent", "127.0.0.1")
	require.NoError(t, err)
	require.NotEmpty(t, result.AccessToken)
	require.NotEmpty(t, result.RefreshToken)
	require.Equal(t, "person@example.com", repo.user.Email)
	require.Equal(t, "Person", repo.user.Name)
	require.Len(t, repo.sessions, 1)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(repo.user.PasswordHash), []byte(strongPassword)))
}

func TestRegisterRejectsWeakPasswords(t *testing.T) {
	t.Parallel()
	tests := []string{"short", "alllowercasebutlong9!", "ALLUPPERCASEBUTLONG9!", "NoNumberIncluded!", "NoSymbolIncluded9"}
	for _, password := range tests {
		_, err := newService(newFakeRepository()).Register(context.Background(), "person@example.com", "Person", password, "", "")
		var appErr *apperror.Error
		require.ErrorAs(t, err, &appErr)
		require.Equal(t, "weak_password", appErr.Code)
	}
}

func TestRegisterRejectsDuplicateEmail(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	repo.createErr = &pgconn.PgError{Code: "23505"}
	_, err := newService(repo).Register(context.Background(), "person@example.com", "Person", strongPassword, "", "")
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, 409, appErr.Status)
	require.Equal(t, "email_taken", appErr.Code)
}

func TestLoginDoesNotRevealWhetherEmailExists(t *testing.T) {
	t.Parallel()
	hash, err := bcrypt.GenerateFromPassword([]byte(strongPassword), bcrypt.MinCost)
	require.NoError(t, err)
	tests := []struct {
		name     string
		repo     *fakeRepository
		password string
	}{
		{name: "unknown email", repo: newFakeRepository(), password: strongPassword},
		{name: "wrong password", repo: &fakeRepository{user: usermodel.User{ID: uuid.New(), Email: "person@example.com", PasswordHash: string(hash)}, sessions: make(map[string]*authmodel.Session)}, password: "Wrong-Password-9!"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newService(tt.repo).Login(context.Background(), "PERSON@example.com", tt.password, "", "")
			var appErr *apperror.Error
			require.True(t, errors.As(err, &appErr))
			require.Equal(t, "invalid_credentials", appErr.Code)
		})
	}
}

func TestRefreshRotatesToken(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	svc := newService(repo)
	registered, err := svc.Register(context.Background(), "person@example.com", "Person", strongPassword, "", "")
	require.NoError(t, err)

	refreshed, err := svc.Refresh(context.Background(), registered.RefreshToken, "", "")
	require.NoError(t, err)
	require.NotEqual(t, registered.RefreshToken, refreshed.RefreshToken)
	require.NotNil(t, repo.sessions[string(sharedauth.HashRefreshToken(registered.RefreshToken))].RevokedAt)
	require.Nil(t, repo.sessions[string(sharedauth.HashRefreshToken(refreshed.RefreshToken))].RevokedAt)
}

func TestRefreshReuseRevokesTokenFamily(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	svc := newService(repo)
	registered, err := svc.Register(context.Background(), "person@example.com", "Person", strongPassword, "", "")
	require.NoError(t, err)
	refreshed, err := svc.Refresh(context.Background(), registered.RefreshToken, "", "")
	require.NoError(t, err)

	_, err = svc.Refresh(context.Background(), registered.RefreshToken, "", "")
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, 401, appErr.Status)
	require.Equal(t, "invalid_refresh_token", appErr.Code)
	require.NotNil(t, repo.sessions[string(sharedauth.HashRefreshToken(refreshed.RefreshToken))].RevokedAt)
}

func TestLogoutRevokesCurrentSessionFamily(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	svc := newService(repo)
	registered, err := svc.Register(context.Background(), "person@example.com", "Person", strongPassword, "", "")
	require.NoError(t, err)
	refreshed, err := svc.Refresh(context.Background(), registered.RefreshToken, "", "")
	require.NoError(t, err)

	require.NoError(t, svc.Logout(context.Background(), refreshed.RefreshToken))
	for _, session := range repo.sessions {
		require.NotNil(t, session.RevokedAt)
	}
	_, err = svc.Refresh(context.Background(), refreshed.RefreshToken, "", "")
	require.Error(t, err)
}

func TestAuthenticationSuccessesEmitAuditEventsWithoutSecrets(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	auditor := &recordingAuthAuditor{}
	tokens := sharedauth.NewTokenManager("this-is-a-long-enough-test-secret-value", "teamops-api", "teamops-web", time.Minute)
	service := New(repo, repo, &fakeTransactor{}, tokens, time.Hour, bcrypt.MinCost, auditor)

	registered, err := service.Register(context.Background(), "person@example.com", "Person", strongPassword, "agent", "192.0.2.1")
	require.NoError(t, err)
	loggedIn, err := service.Login(context.Background(), "person@example.com", strongPassword, "agent", "192.0.2.1")
	require.NoError(t, err)
	require.NoError(t, service.Logout(context.Background(), loggedIn.RefreshToken))
	require.NotEmpty(t, registered.RefreshToken)
	require.Equal(t, []string{"user.registered", "auth.login", "auth.logout"}, auditor.actions)
}

func TestSessionMetadataIsValidAndBounded(t *testing.T) {
	t.Parallel()
	agent, ip := sanitizeSessionMetadata(strings.Repeat("界", 600), "not-an-ip")
	require.True(t, utf8.ValidString(agent))
	require.Equal(t, 512, utf8.RuneCountInString(agent))
	require.Empty(t, ip)

	_, ip = sanitizeSessionMetadata("agent", "2001:0db8::1")
	require.Equal(t, "2001:db8::1", ip)
}
