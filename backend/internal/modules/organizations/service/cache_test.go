package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/example/teamops/backend/internal/cache"
	orgguard "github.com/example/teamops/backend/internal/modules/organizations/guard"
	orgmodel "github.com/example/teamops/backend/internal/modules/organizations/model"
	"github.com/example/teamops/backend/internal/shared/pagination"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type organizationCacheStore struct {
	mu        sync.Mutex
	values    map[string][]byte
	getErr    error
	setErr    error
	deleteErr error
	sets      int
	deletes   int
}

func (s *organizationCacheStore) GetJSON(_ context.Context, key string, dst any) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.getErr != nil {
		return false, s.getErr
	}
	raw, ok := s.values[key]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(raw, dst)
}

func (s *organizationCacheStore) SetJSON(_ context.Context, key string, value any, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sets++
	if s.setErr != nil {
		return s.setErr
	}
	raw, err := json.Marshal(value)
	if err == nil {
		s.values[key] = raw
	}
	return err
}

func (s *organizationCacheStore) Delete(_ context.Context, keys ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletes++
	if s.deleteErr != nil {
		return s.deleteErr
	}
	for _, key := range keys {
		delete(s.values, key)
	}
	return nil
}

type cacheOrganizationRepository struct {
	organization orgmodel.Organization
	membership   orgmodel.Membership
	previousRole *orgmodel.Role
	removedRole  orgmodel.Role
	gets         int
	updates      int
	deletes      int
}

func (r *cacheOrganizationRepository) Create(context.Context, uuid.UUID, string, string) (orgmodel.Organization, error) {
	return orgmodel.Organization{}, nil
}
func (r *cacheOrganizationRepository) ListForUser(context.Context, uuid.UUID, pagination.Params) ([]orgmodel.Organization, int64, error) {
	return nil, 0, nil
}
func (r *cacheOrganizationRepository) Get(context.Context, uuid.UUID) (orgmodel.Organization, error) {
	r.gets++
	return r.organization, nil
}
func (r *cacheOrganizationRepository) Update(_ context.Context, _ uuid.UUID, name, slug string, version int) (orgmodel.Organization, error) {
	r.updates++
	r.organization.Name = name
	r.organization.Slug = slug
	r.organization.Version = version + 1
	return r.organization, nil
}
func (r *cacheOrganizationRepository) Delete(context.Context, uuid.UUID) error {
	r.deletes++
	return nil
}
func (r *cacheOrganizationRepository) ListMembers(context.Context, uuid.UUID) ([]orgmodel.Membership, error) {
	return nil, nil
}
func (r *cacheOrganizationRepository) AddMember(context.Context, uuid.UUID, string, orgmodel.Role) (orgmodel.Membership, *orgmodel.Role, error) {
	return r.membership, r.previousRole, nil
}
func (r *cacheOrganizationRepository) RemoveMember(context.Context, uuid.UUID, uuid.UUID) (orgmodel.Role, error) {
	return r.removedRole, nil
}

type cacheAuditor struct{}

func (cacheAuditor) Record(context.Context, uuid.UUID, uuid.UUID, string, string, string, string, map[string]any) error {
	return nil
}

type recordingOrganizationAuditor struct {
	actions  []string
	metadata []map[string]any
}

func (a *recordingOrganizationAuditor) Record(_ context.Context, _, _ uuid.UUID, action, _, _, _ string, metadata map[string]any) error {
	a.actions = append(a.actions, action)
	a.metadata = append(a.metadata, metadata)
	return nil
}

func TestOrganizationCacheAsideHitMissAndInvalidation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	organizationID, userID := uuid.New(), uuid.New()
	repository := &cacheOrganizationRepository{organization: orgmodel.Organization{
		ID: organizationID, Name: "TeamOps", Slug: "teamops", Version: 1,
	}}
	store := &organizationCacheStore{values: make(map[string][]byte)}
	guard := orgguard.New(authorizationMemberships{roles: map[uuid.UUID]orgmodel.Role{userID: orgmodel.RoleOwner}})
	service := New(repository, cacheAuditor{}, nil, guard, store, time.Minute)

	for range 2 {
		organization, err := service.Get(ctx, userID, organizationID)
		require.NoError(t, err)
		require.Equal(t, "TeamOps", organization.Name)
		require.Equal(t, orgmodel.RoleOwner, organization.Role)
	}
	require.Equal(t, 1, repository.gets)
	require.Equal(t, 1, store.sets)

	_, err := service.Update(ctx, userID, organizationID, "Updated", "updated", 1, "request")
	require.NoError(t, err)
	require.Equal(t, 1, store.deletes)
	_, exists := store.values[cache.OrganizationKey(organizationID.String())]
	require.False(t, exists)

	_, err = service.Get(ctx, userID, organizationID)
	require.NoError(t, err)
	require.Equal(t, 2, repository.gets)

	require.NoError(t, service.Delete(ctx, userID, organizationID, "request"))
	require.Equal(t, 2, store.deletes)
}

func TestOrganizationCacheFailureDoesNotFailReadsOrWrites(t *testing.T) {
	t.Parallel()
	organizationID, userID := uuid.New(), uuid.New()
	repository := &cacheOrganizationRepository{organization: orgmodel.Organization{
		ID: organizationID, Name: "Available", Slug: "available", Version: 1,
	}}
	store := &organizationCacheStore{
		values: make(map[string][]byte), getErr: errors.New("redis unavailable"),
		setErr: errors.New("redis unavailable"), deleteErr: errors.New("redis unavailable"),
	}
	guard := orgguard.New(authorizationMemberships{roles: map[uuid.UUID]orgmodel.Role{userID: orgmodel.RoleOwner}})
	service := New(repository, cacheAuditor{}, nil, guard, store, time.Minute)

	organization, err := service.Get(context.Background(), userID, organizationID)
	require.NoError(t, err)
	require.Equal(t, "Available", organization.Name)

	_, err = service.Update(context.Background(), userID, organizationID, "Still available", "still-available", 1, "request")
	require.NoError(t, err)
}

func TestRoleChangeAndMemberRemovalEmitDistinctAuditEvents(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	organizationID, adminID, memberID := uuid.New(), uuid.New(), uuid.New()
	oldRole := orgmodel.RoleViewer
	repository := &cacheOrganizationRepository{
		membership:   orgmodel.Membership{OrganizationID: organizationID, UserID: memberID, Role: orgmodel.RoleMember},
		previousRole: &oldRole,
		removedRole:  orgmodel.RoleMember,
	}
	store := &organizationCacheStore{values: make(map[string][]byte)}
	guard := orgguard.New(authorizationMemberships{roles: map[uuid.UUID]orgmodel.Role{adminID: orgmodel.RoleAdmin}})
	auditor := &recordingOrganizationAuditor{}
	service := New(repository, auditor, nil, guard, store, time.Minute)

	_, err := service.AddMember(ctx, adminID, organizationID, "member@example.com", orgmodel.RoleMember, "request")
	require.NoError(t, err)
	require.NoError(t, service.RemoveMember(ctx, adminID, organizationID, memberID, "request"))
	require.Equal(t, []string{"member.role_changed", "member.removed"}, auditor.actions)
	require.Equal(t, oldRole, auditor.metadata[0]["oldRole"])
	require.Equal(t, orgmodel.RoleMember, auditor.metadata[0]["newRole"])
}
