package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/example/teamops/backend/internal/database"
	"github.com/example/teamops/backend/internal/modules/tasks/model"
	"github.com/example/teamops/backend/internal/shared/pagination"
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
func scanTask(row pgx.Row) (model.Task, error) {
	var t model.Task
	err := row.Scan(&t.ID, &t.OrganizationID, &t.ProjectID, &t.AssigneeID, &t.CreatedBy, &t.Title, &t.Description, &t.Status, &t.Priority, &t.DueAt, &t.Version, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}
func (r *Repository) Create(ctx context.Context, t model.Task) (model.Task, error) {
	return scanTask(database.Executor(ctx, r.db).QueryRow(ctx, `INSERT INTO tasks(organization_id,project_id,assignee_id,created_by,title,description,status,priority,due_date) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id,organization_id,project_id,assignee_id,created_by,title,description,status,priority,due_date,version,created_at,updated_at`, t.OrganizationID, t.ProjectID, t.AssigneeID, t.CreatedBy, t.Title, t.Description, t.Status, t.Priority, t.DueAt))
}
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (model.Task, error) {
	return scanTask(database.Executor(ctx, r.db).QueryRow(ctx, `SELECT id,organization_id,project_id,assignee_id,created_by,title,description,status,priority,due_date,version,created_at,updated_at FROM tasks WHERE id=$1`, id))
}
func (r *Repository) List(ctx context.Context, projectID uuid.UUID, p pagination.Params, f model.Filters) ([]model.Task, int64, error) {
	clauses := []string{"project_id=$1"}
	args := []any{projectID}
	add := func(v any, clause string) {
		args = append(args, v)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if f.Status != "" {
		add(f.Status, "status=$%d")
	}
	if f.Priority != "" {
		add(f.Priority, "priority=$%d")
	}
	if f.AssigneeID != nil {
		add(*f.AssigneeID, "assignee_id=$%d")
	}
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		position := len(args)
		clauses = append(clauses, fmt.Sprintf("(title ILIKE $%d OR description ILIKE $%d)", position, position))
	}
	args = append(args, p.PageSize, p.Offset)
	limitPos := len(args) - 1
	sort := map[string]string{"createdAt": "created_at", "updatedAt": "updated_at", "dueAt": "due_date", "priority": "priority", "title": "title"}[p.SortBy]
	q := fmt.Sprintf(`SELECT id,organization_id,project_id,assignee_id,created_by,title,description,status,priority,due_date,version,created_at,updated_at,count(*) OVER() FROM tasks WHERE %s ORDER BY %s %s NULLS LAST LIMIT $%d OFFSET $%d`, strings.Join(clauses, " AND "), sort, p.Order, limitPos, limitPos+1)
	rows, err := database.Executor(ctx, r.db).Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []model.Task{}
	var total int64
	for rows.Next() {
		var t model.Task
		if err := rows.Scan(&t.ID, &t.OrganizationID, &t.ProjectID, &t.AssigneeID, &t.CreatedBy, &t.Title, &t.Description, &t.Status, &t.Priority, &t.DueAt, &t.Version, &t.CreatedAt, &t.UpdatedAt, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, t)
	}
	return out, total, rows.Err()
}

func (r *Repository) Update(ctx context.Context, t model.Task) (model.Task, error) {
	return scanTask(database.Executor(ctx, r.db).QueryRow(ctx, `UPDATE tasks SET assignee_id=$2,title=$3,description=$4,status=$5,priority=$6,due_date=$7,version=version+1,updated_at=now() WHERE id=$1 AND version=$8 RETURNING id,organization_id,project_id,assignee_id,created_by,title,description,status,priority,due_date,version,created_at,updated_at`, t.ID, t.AssigneeID, t.Title, t.Description, t.Status, t.Priority, t.DueAt, t.Version))
}
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := database.Executor(ctx, r.db).Exec(ctx, `DELETE FROM tasks WHERE id=$1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}
func (r *Repository) AddComment(ctx context.Context, taskID, authorID uuid.UUID, body string) (model.Comment, error) {
	var c model.Comment
	err := database.Executor(ctx, r.db).QueryRow(ctx, `INSERT INTO task_comments(task_id,author_id,body) VALUES($1,$2,$3) RETURNING id,task_id,author_id,body,created_at,updated_at`, taskID, authorID, body).Scan(&c.ID, &c.TaskID, &c.AuthorID, &c.Body, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}
func (r *Repository) Comments(ctx context.Context, taskID uuid.UUID) ([]model.Comment, error) {
	rows, err := database.Executor(ctx, r.db).Query(ctx, `SELECT id,task_id,author_id,body,created_at,updated_at FROM task_comments WHERE task_id=$1 ORDER BY created_at`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Comment{}
	for rows.Next() {
		var c model.Comment
		if err := rows.Scan(&c.ID, &c.TaskID, &c.AuthorID, &c.Body, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (r *Repository) CreateLabel(ctx context.Context, orgID uuid.UUID, name, color string) (model.Label, error) {
	var l model.Label
	err := database.Executor(ctx, r.db).QueryRow(ctx, `INSERT INTO task_labels(organization_id,name,color) VALUES($1,$2,$3) RETURNING id,organization_id,name,color`, orgID, name, color).Scan(&l.ID, &l.OrganizationID, &l.Name, &l.Color)
	return l, err
}
func (r *Repository) Labels(ctx context.Context, orgID uuid.UUID) ([]model.Label, error) {
	rows, err := database.Executor(ctx, r.db).Query(ctx, `SELECT id,organization_id,name,color FROM task_labels WHERE organization_id=$1 ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Label{}
	for rows.Next() {
		var l model.Label
		if err := rows.Scan(&l.ID, &l.OrganizationID, &l.Name, &l.Color); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
func (r *Repository) SetLabels(ctx context.Context, taskID, orgID uuid.UUID, labelIDs []uuid.UUID) error {
	return r.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		db := database.Executor(txCtx, r.db)
		if _, err := db.Exec(txCtx, `DELETE FROM task_label_assignments WHERE task_id=$1`, taskID); err != nil {
			return err
		}
		for _, id := range labelIDs {
			tag, err := db.Exec(txCtx, `INSERT INTO task_label_assignments(task_id,task_label_id,organization_id) SELECT $1,id,$3 FROM task_labels WHERE id=$2 AND organization_id=$3`, taskID, id, orgID)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return fmt.Errorf("label %s not in organization", id)
			}
		}
		return nil
	})
}
