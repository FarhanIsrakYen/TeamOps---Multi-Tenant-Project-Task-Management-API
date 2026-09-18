package repository

import (
	"context"
	"fmt"

	"github.com/example/teamops/backend/internal/database"
	"github.com/example/teamops/backend/internal/modules/projects/model"
	"github.com/example/teamops/backend/internal/shared/pagination"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Repository { return &Repository{db: db} }
func scanProject(row pgx.Row) (model.Project, error) {
	var p model.Project
	err := row.Scan(&p.ID, &p.OrganizationID, &p.Name, &p.Description, &p.Archived, &p.Version, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}
func (r *Repository) Create(ctx context.Context, orgID uuid.UUID, name, description string) (model.Project, error) {
	return scanProject(database.Executor(ctx, r.db).QueryRow(ctx, `INSERT INTO projects(organization_id,name,description) VALUES($1,$2,$3) RETURNING id,organization_id,name,description,archived,version,created_at,updated_at`, orgID, name, description))
}
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (model.Project, error) {
	return scanProject(database.Executor(ctx, r.db).QueryRow(ctx, `SELECT id,organization_id,name,description,archived,version,created_at,updated_at FROM projects WHERE id=$1`, id))
}
func (r *Repository) List(ctx context.Context, orgID uuid.UUID, p pagination.Params, filters model.Filters) ([]model.Project, int64, error) {
	sort := map[string]string{"name": "name", "created_at": "created_at", "updated_at": "updated_at"}[p.SortBy]
	where := "organization_id=$1"
	args := []any{orgID}
	if filters.Archived != nil {
		args = append(args, *filters.Archived)
		where += fmt.Sprintf(" AND archived=$%d", len(args))
	}
	if filters.Search != "" {
		args = append(args, "%"+filters.Search+"%")
		position := len(args)
		where += fmt.Sprintf(" AND (name ILIKE $%d OR description ILIKE $%d)", position, position)
	}
	if sort == "" {
		sort = "created_at"
	}
	order := "DESC"
	if p.Order == "asc" {
		order = "ASC"
	}
	filterArgCount := len(args)
	args = append(args, p.PageSize, p.Offset)
	limitPosition := len(args) - 1
	q := fmt.Sprintf(`SELECT id,organization_id,name,description,archived,version,created_at,updated_at,count(*) OVER() FROM projects WHERE %s ORDER BY %s %s,id %s LIMIT $%d OFFSET $%d`, where, sort, order, order, limitPosition, limitPosition+1)
	rows, err := database.Executor(ctx, r.db).Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []model.Project{}
	var total int64
	for rows.Next() {
		var x model.Project
		if err := rows.Scan(&x.ID, &x.OrganizationID, &x.Name, &x.Description, &x.Archived, &x.Version, &x.CreatedAt, &x.UpdatedAt, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(out) == 0 && p.Offset > 0 {
		err = database.Executor(ctx, r.db).QueryRow(ctx, `SELECT count(*) FROM projects WHERE `+where, args[:filterArgCount]...).Scan(&total)
		if err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}
func (r *Repository) Update(ctx context.Context, id uuid.UUID, name, description string, archived bool, version int) (model.Project, error) {
	return scanProject(database.Executor(ctx, r.db).QueryRow(ctx, `UPDATE projects SET name=$2,description=$3,archived=$4,version=version+1,updated_at=now() WHERE id=$1 AND version=$5 RETURNING id,organization_id,name,description,archived,version,created_at,updated_at`, id, name, description, archived, version))
}
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := database.Executor(ctx, r.db).Exec(ctx, `DELETE FROM projects WHERE id=$1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}
