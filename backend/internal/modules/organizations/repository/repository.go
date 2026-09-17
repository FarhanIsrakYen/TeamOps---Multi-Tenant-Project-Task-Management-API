package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/example/teamops/backend/internal/database"
	"github.com/example/teamops/backend/internal/modules/organizations/model"
	usermodel "github.com/example/teamops/backend/internal/modules/users/model"
	"github.com/example/teamops/backend/internal/shared/pagination"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
	tx database.Transactor
}

var ErrOwnerRoleImmutable = errors.New("owner role cannot be changed")

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db, tx: database.NewTransactor(db)}
}

func (r *Repository) Create(ctx context.Context, userID uuid.UUID, name, slug string) (model.Organization, error) {
	var o model.Organization
	err := r.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		db := database.Executor(txCtx, r.db)
		if err := db.QueryRow(txCtx, `INSERT INTO organizations(name,slug,created_by) VALUES($1,$2,$3) RETURNING id,name,slug,created_by,version,created_at,updated_at`, name, slug, userID).Scan(&o.ID, &o.Name, &o.Slug, &o.CreatedBy, &o.Version, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return fmt.Errorf("insert organization: %w", err)
		}
		_, err := db.Exec(txCtx, `INSERT INTO organization_members(organization_id,user_id,role) VALUES($1,$2,'OWNER')`, o.ID, userID)
		return err
	})
	return o, err
}

func (r *Repository) ListForUser(ctx context.Context, userID uuid.UUID, p pagination.Params) ([]model.Organization, int64, error) {
	order := map[string]string{"name": "o.name", "createdAt": "o.created_at"}[p.SortBy]
	q := fmt.Sprintf(`SELECT o.id,o.name,o.slug,o.created_by,o.version,m.role,o.created_at,o.updated_at,count(*) OVER() FROM organizations o JOIN organization_members m ON m.organization_id=o.id WHERE m.user_id=$1 ORDER BY %s %s LIMIT $2 OFFSET $3`, order, p.Order)
	rows, err := database.Executor(ctx, r.db).Query(ctx, q, userID, p.PageSize, p.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []model.Organization{}
	var total int64
	for rows.Next() {
		var o model.Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.Slug, &o.CreatedBy, &o.Version, &o.Role, &o.CreatedAt, &o.UpdatedAt, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, o)
	}
	return out, total, rows.Err()
}

func (r *Repository) GetForUser(ctx context.Context, id, userID uuid.UUID) (model.Organization, error) {
	var o model.Organization
	err := database.Executor(ctx, r.db).QueryRow(ctx, `SELECT o.id,o.name,o.slug,o.created_by,o.version,m.role,o.created_at,o.updated_at FROM organizations o JOIN organization_members m ON m.organization_id=o.id WHERE o.id=$1 AND m.user_id=$2`, id, userID).Scan(&o.ID, &o.Name, &o.Slug, &o.CreatedBy, &o.Version, &o.Role, &o.CreatedAt, &o.UpdatedAt)
	return o, err
}

func (r *Repository) Role(ctx context.Context, orgID, userID uuid.UUID) (model.Role, error) {
	var role model.Role
	err := database.Executor(ctx, r.db).QueryRow(ctx, `SELECT role FROM organization_members WHERE organization_id=$1 AND user_id=$2`, orgID, userID).Scan(&role)
	return role, err
}

func (r *Repository) Update(ctx context.Context, id uuid.UUID, name, slug string, version int) (model.Organization, error) {
	var o model.Organization
	err := database.Executor(ctx, r.db).QueryRow(ctx, `UPDATE organizations SET name=$2,slug=$3,version=version+1,updated_at=now() WHERE id=$1 AND version=$4 RETURNING id,name,slug,created_by,version,created_at,updated_at`, id, name, slug, version).Scan(&o.ID, &o.Name, &o.Slug, &o.CreatedBy, &o.Version, &o.CreatedAt, &o.UpdatedAt)
	return o, err
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := database.Executor(ctx, r.db).Exec(ctx, `DELETE FROM organizations WHERE id=$1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

func (r *Repository) ListMembers(ctx context.Context, orgID uuid.UUID) ([]model.Membership, error) {
	rows, err := database.Executor(ctx, r.db).Query(ctx, `SELECT m.organization_id,m.user_id,m.role,m.created_at,u.id,u.email,u.name,u.created_at,u.updated_at FROM organization_members m JOIN users u ON u.id=m.user_id WHERE m.organization_id=$1 ORDER BY m.created_at`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Membership{}
	for rows.Next() {
		var m model.Membership
		var u usermodel.User
		if err := rows.Scan(&m.OrganizationID, &m.UserID, &m.Role, &m.CreatedAt, &u.ID, &u.Email, &u.Name, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		m.User = &u
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *Repository) AddMember(ctx context.Context, orgID uuid.UUID, email string, role model.Role) (model.Membership, error) {
	var m model.Membership
	var currentRole model.Role
	db := database.Executor(ctx, r.db)
	err := db.QueryRow(ctx, `SELECT m.role FROM organization_members m JOIN users u ON u.id=m.user_id WHERE m.organization_id=$1 AND u.email=lower($2)`, orgID, email).Scan(&currentRole)
	if err == nil && currentRole == model.RoleOwner {
		return m, ErrOwnerRoleImmutable
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return m, err
	}
	err = db.QueryRow(ctx, `INSERT INTO organization_members(organization_id,user_id,role) SELECT $1,id,$3 FROM users WHERE email=lower($2) ON CONFLICT(organization_id,user_id) DO UPDATE SET role=EXCLUDED.role WHERE organization_members.role <> 'OWNER' RETURNING organization_id,user_id,role,created_at`, orgID, email, role).Scan(&m.OrganizationID, &m.UserID, &m.Role, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, pgx.ErrNoRows
	}
	return m, err
}
