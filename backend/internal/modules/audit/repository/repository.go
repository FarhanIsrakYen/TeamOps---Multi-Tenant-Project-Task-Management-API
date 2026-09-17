package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/example/teamops/backend/internal/database"
	"github.com/example/teamops/backend/internal/modules/audit/model"
	"github.com/example/teamops/backend/internal/shared/pagination"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Repository { return &Repository{db: db} }
func (r *Repository) Record(ctx context.Context, orgID, actorID uuid.UUID, action, kind, resourceID, requestID string, metadata map[string]any) error {
	if metadata == nil {
		metadata = map[string]any{}
	}
	raw, _ := json.Marshal(metadata)
	_, err := database.Executor(ctx, r.db).Exec(ctx, `INSERT INTO audit_logs(organization_id,actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,$2,$3,$4,$5,$6,$7)`, orgID, actorID, action, kind, resourceID, requestID, raw)
	if err != nil {
		return fmt.Errorf("record audit log: %w", err)
	}
	return nil
}
func (r *Repository) List(ctx context.Context, orgID uuid.UUID, p pagination.Params) ([]model.AuditLog, int64, error) {
	q := `SELECT id,organization_id,actor_id,action,resource_type,resource_id,metadata,request_id,created_at,count(*) OVER() FROM audit_logs WHERE organization_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`
	rows, err := database.Executor(ctx, r.db).Query(ctx, q, orgID, p.PageSize, p.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []model.AuditLog{}
	var total int64
	for rows.Next() {
		var x model.AuditLog
		var raw []byte
		if err := rows.Scan(&x.ID, &x.OrganizationID, &x.ActorID, &x.Action, &x.ResourceType, &x.ResourceID, &raw, &x.RequestID, &x.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal(raw, &x.Metadata)
		out = append(out, x)
	}
	return out, total, rows.Err()
}
