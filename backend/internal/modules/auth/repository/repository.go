package repository

import (
	"context"
	"errors"
	"time"

	"github.com/example/teamops/backend/internal/database"
	authmodel "github.com/example/teamops/backend/internal/modules/auth/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
	tx database.Transactor
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db, tx: database.NewTransactor(db)}
}
func (r *Repository) CreateSession(ctx context.Context, s authmodel.Session, agent, ip string) error {
	_, err := database.Executor(ctx, r.db).Exec(ctx, `INSERT INTO refresh_tokens(id,family_id,user_id,token_hash,expires_at,user_agent,ip_address) VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,'')::inet)`, s.ID, s.FamilyID, s.UserID, s.TokenHash, s.ExpiresAt, agent, ip)
	return err
}
func (r *Repository) RotateSession(ctx context.Context, hash []byte, next authmodel.Session, agent, ip string) (authmodel.Session, error) {
	var old authmodel.Session
	reused := false
	err := r.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		db := database.Executor(txCtx, r.db)
		if err := db.QueryRow(txCtx, `SELECT id,family_id,user_id,token_hash,expires_at,revoked_at FROM refresh_tokens WHERE token_hash=$1 FOR UPDATE`, hash).Scan(&old.ID, &old.FamilyID, &old.UserID, &old.TokenHash, &old.ExpiresAt, &old.RevokedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return authmodel.ErrRefreshSessionInvalid
			}
			return err
		}
		if old.RevokedAt != nil || time.Now().After(old.ExpiresAt) {
			reused = true
			_, err := db.Exec(txCtx, `UPDATE refresh_tokens SET revoked_at=COALESCE(revoked_at,now()) WHERE family_id=$1`, old.FamilyID)
			return err
		}
		next.FamilyID = old.FamilyID
		next.UserID = old.UserID
		tag, err := db.Exec(txCtx, `UPDATE refresh_tokens SET revoked_at=now(),replaced_by=$2 WHERE id=$1 AND revoked_at IS NULL`, old.ID, next.ID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return authmodel.ErrRefreshSessionInvalid
		}
		_, err = db.Exec(txCtx, `INSERT INTO refresh_tokens(id,family_id,user_id,token_hash,expires_at,user_agent,ip_address) VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,'')::inet)`, next.ID, next.FamilyID, next.UserID, next.TokenHash, next.ExpiresAt, agent, ip)
		return err
	})
	if err != nil {
		return old, err
	}
	if reused {
		return old, authmodel.ErrRefreshSessionInvalid
	}
	return old, nil
}
func (r *Repository) RevokeSession(ctx context.Context, hash []byte) (uuid.UUID, error) {
	var userID uuid.UUID
	err := database.Executor(ctx, r.db).QueryRow(ctx, `WITH target AS (
		SELECT family_id,user_id FROM refresh_tokens WHERE token_hash=$1
	), revoked AS (
		UPDATE refresh_tokens SET revoked_at=COALESCE(revoked_at,now())
		WHERE family_id=(SELECT family_id FROM target)
	)
	SELECT user_id FROM target`, hash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, nil
	}
	return userID, err
}
func (r *Repository) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	tag, err := database.Executor(ctx, r.db).Exec(ctx, `DELETE FROM refresh_tokens WHERE expires_at < now()-interval '1 day'`)
	return tag.RowsAffected(), err
}
