package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/example/teamops/backend/internal/database"
	"github.com/example/teamops/backend/internal/modules/audit/model"
	"github.com/example/teamops/backend/internal/shared/pagination"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Repository { return &Repository{db: db} }
func (r *Repository) Insert(ctx context.Context, entry model.Entry) error {
	if entry.Metadata == nil {
		entry.Metadata = map[string]any{}
	}
	raw, err := json.Marshal(entry.Metadata)
	if err != nil {
		return fmt.Errorf("encode audit metadata: %w", err)
	}
	_, err = database.Executor(ctx, r.db).Exec(ctx, `INSERT INTO audit_logs(organization_id,actor_user_id,action,resource_type,resource_id,request_id,metadata,ip_address,user_agent) VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::inet,$9)`, entry.OrganizationID, entry.ActorUserID, entry.Action, entry.ResourceType, entry.ResourceID, entry.RequestID, raw, entry.IPAddress, entry.UserAgent)
	if err != nil {
		return fmt.Errorf("record audit log: %w", err)
	}
	return nil
}
func (r *Repository) List(ctx context.Context, orgID uuid.UUID, p pagination.Params, filters model.Filters) ([]model.AuditLog, int64, error) {
	clauses := []string{"organization_id=$1"}
	args := []any{orgID}
	add := func(value any, expression string) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(expression, len(args)))
	}
	if filters.ActorUserID != nil {
		add(*filters.ActorUserID, "actor_user_id=$%d")
	}
	if filters.Action != "" {
		add(filters.Action, "action=$%d")
	}
	if filters.ResourceType != "" {
		add(filters.ResourceType, "resource_type=$%d")
	}
	if filters.ResourceID != "" {
		add(filters.ResourceID, "resource_id=$%d")
	}
	if filters.From != nil {
		add(*filters.From, "created_at >= $%d")
	}
	if filters.To != nil {
		add(*filters.To, "created_at <= $%d")
	}
	filterArgs := len(args)
	args = append(args, p.PageSize, p.Offset)
	q := fmt.Sprintf(`SELECT id,organization_id,actor_user_id,action,resource_type,resource_id,metadata,request_id,COALESCE(host(ip_address),''),user_agent,created_at,count(*) OVER() FROM audit_logs WHERE %s ORDER BY created_at DESC,id DESC LIMIT $%d OFFSET $%d`, strings.Join(clauses, " AND "), filterArgs+1, filterArgs+2)
	rows, err := database.Executor(ctx, r.db).Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []model.AuditLog{}
	var total int64
	for rows.Next() {
		var x model.AuditLog
		var raw []byte
		if err := rows.Scan(&x.ID, &x.OrganizationID, &x.ActorUserID, &x.Action, &x.ResourceType, &x.ResourceID, &raw, &x.RequestID, &x.IPAddress, &x.UserAgent, &x.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal(raw, &x.Metadata)
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(out) == 0 && p.Offset > 0 {
		countQuery := `SELECT count(*) FROM audit_logs WHERE ` + strings.Join(clauses, " AND ")
		if err = database.Executor(ctx, r.db).QueryRow(ctx, countQuery, args[:filterArgs]...).Scan(&total); err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}
