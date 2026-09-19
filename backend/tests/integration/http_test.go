//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const integrationPassword = "Strong-Password-123!"

type tokenData struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	User         struct {
		ID    uuid.UUID `json:"id"`
		Email string    `json:"email"`
	} `json:"user"`
}

type organizationData struct {
	ID      uuid.UUID `json:"id"`
	Version int       `json:"version"`
}

type projectData struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`
	Version        int       `json:"version"`
}

type taskData struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"projectId"`
	Title     string    `json:"title"`
	Version   int       `json:"version"`
}

func TestHTTPAuthenticationAuthorizationCRUDAndIDOR(t *testing.T) {
	f := newHTTPFixture(t)
	ownerA := registerUser(t, f, "owner-a@example.com", "Owner A")
	ownerB := registerUser(t, f, "owner-b@example.com", "Owner B")
	admin := registerUser(t, f, "admin@example.com", "Admin")
	member := registerUser(t, f, "member@example.com", "Member")
	viewer := registerUser(t, f, "viewer@example.com", "Viewer")

	organizationA := createOrganization(t, f, ownerA.AccessToken, "Organization A", "organization-a")
	organizationB := createOrganization(t, f, ownerB.AccessToken, "Organization B", "organization-b")
	addMember(t, f, ownerA.AccessToken, organizationA.ID, admin.User.Email, "ADMIN")
	addMember(t, f, ownerA.AccessToken, organizationA.ID, member.User.Email, "MEMBER")
	addMember(t, f, ownerA.AccessToken, organizationA.ID, viewer.User.Email, "VIEWER")

	projectA := createProject(t, f, ownerA.AccessToken, organizationA.ID, "Project A")
	projectB := createProject(t, f, ownerB.AccessToken, organizationB.ID, "Project B")
	task := createTask(t, f, ownerA.AccessToken, projectA.ID, "Concurrent Task")

	t.Run("security boundaries", func(t *testing.T) {
		tests := []struct {
			name, method, path, token string
			body                      any
			wantStatus                int
		}{
			{name: "user A cannot read user B organization", method: http.MethodGet, path: "/api/v1/organizations/" + organizationB.ID.String(), token: ownerA.AccessToken, wantStatus: http.StatusNotFound},
			{name: "organization A member cannot read organization B project", method: http.MethodGet, path: "/api/v1/projects/" + projectB.ID.String(), token: member.AccessToken, wantStatus: http.StatusForbidden},
			{name: "viewer cannot modify task", method: http.MethodPatch, path: "/api/v1/tasks/" + task.ID.String(), token: viewer.AccessToken, body: map[string]any{"priority": "HIGH", "version": task.Version}, wantStatus: http.StatusForbidden},
			{name: "member cannot modify membership", method: http.MethodPost, path: "/api/v1/organizations/" + organizationA.ID.String() + "/members", token: member.AccessToken, body: map[string]any{"email": ownerB.User.Email, "role": "VIEWER"}, wantStatus: http.StatusForbidden},
			{name: "admin cannot delete organization", method: http.MethodDelete, path: "/api/v1/organizations/" + organizationA.ID.String(), token: admin.AccessToken, wantStatus: http.StatusForbidden},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				recorder, response := f.request(t, test.method, test.path, test.token, test.body)
				require.Equal(t, test.wantStatus, recorder.Code, recorder.Body.String())
				require.NotNil(t, response.Error)
				require.Empty(t, response.Data)
			})
		}
	})

	t.Run("project and task CRUD", func(t *testing.T) {
		recorder, response := f.request(t, http.MethodGet, "/api/v1/projects/"+projectA.ID.String(), ownerA.AccessToken, nil)
		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, projectA.ID, decodeData[projectData](t, response).ID)

		recorder, response = f.request(t, http.MethodPatch, "/api/v1/projects/"+projectA.ID.String(), ownerA.AccessToken, map[string]any{"description": "updated", "version": projectA.Version})
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.Equal(t, projectA.Version+1, decodeData[projectData](t, response).Version)

		throwawayProject := createProject(t, f, ownerA.AccessToken, organizationA.ID, "Throwaway Project")
		recorder, _ = f.request(t, http.MethodDelete, "/api/v1/projects/"+throwawayProject.ID.String(), ownerA.AccessToken, nil)
		require.Equal(t, http.StatusNoContent, recorder.Code)

		throwawayTask := createTask(t, f, ownerA.AccessToken, projectA.ID, "Throwaway Task")
		recorder, _ = f.request(t, http.MethodDelete, "/api/v1/tasks/"+throwawayTask.ID.String(), ownerA.AccessToken, nil)
		require.Equal(t, http.StatusNoContent, recorder.Code)
		recorder, response = f.request(t, http.MethodGet, "/api/v1/tasks/"+throwawayTask.ID.String(), ownerA.AccessToken, nil)
		require.Equal(t, http.StatusNotFound, recorder.Code)
		require.Equal(t, "not_found", response.Error.Code)
	})

	t.Run("pagination filtering sorting and search", func(t *testing.T) {
		createTask(t, f, ownerA.AccessToken, projectA.ID, "Task Two")
		createTask(t, f, ownerA.AccessToken, projectA.ID, "Task Three")
		path := fmt.Sprintf("/api/v1/projects/%s/tasks?page=1&page_size=2&status=TODO&priority=MEDIUM&search=Task&sort=-created_at", projectA.ID)
		recorder, response := f.request(t, http.MethodGet, path, ownerA.AccessToken, nil)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		items := decodeData[[]taskData](t, response)
		require.Len(t, items, 2)
		var meta struct {
			Page       int `json:"page"`
			PageSize   int `json:"pageSize"`
			Total      int `json:"total"`
			TotalPages int `json:"totalPages"`
		}
		require.NoError(t, json.Unmarshal(response.Meta, &meta))
		require.Equal(t, 1, meta.Page)
		require.Equal(t, 2, meta.PageSize)
		require.Equal(t, 3, meta.Total)
		require.Equal(t, 2, meta.TotalPages)
	})

	t.Run("consistent HTTP errors", func(t *testing.T) {
		tests := []struct {
			name, path, token, code string
			status                  int
		}{
			{name: "missing authentication", path: "/api/v1/me", code: "unauthorized", status: http.StatusUnauthorized},
			{name: "malformed resource id", path: "/api/v1/tasks/not-a-uuid", token: ownerA.AccessToken, code: "validation_error", status: http.StatusBadRequest},
			{name: "invalid filter", path: "/api/v1/projects/" + projectA.ID.String() + "/tasks?status=NOT_A_STATUS", token: ownerA.AccessToken, code: "invalid_status", status: http.StatusBadRequest},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				recorder, response := f.request(t, http.MethodGet, test.path, test.token, nil)
				require.Equal(t, test.status, recorder.Code)
				require.NotNil(t, response.Error)
				require.Equal(t, test.code, response.Error.Code)
				require.NotEmpty(t, response.Error.RequestID)
			})
		}
	})

	t.Run("revoked refresh token reuse revokes family", func(t *testing.T) {
		rotatedRecorder, rotatedResponse := f.request(t, http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refreshToken": viewer.RefreshToken})
		require.Equal(t, http.StatusOK, rotatedRecorder.Code, rotatedRecorder.Body.String())
		rotated := decodeData[tokenData](t, rotatedResponse)

		reuseRecorder, reuseResponse := f.request(t, http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refreshToken": viewer.RefreshToken})
		require.Equal(t, http.StatusUnauthorized, reuseRecorder.Code)
		require.Equal(t, "invalid_refresh_token", reuseResponse.Error.Code)

		familyRecorder, familyResponse := f.request(t, http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refreshToken": rotated.RefreshToken})
		require.Equal(t, http.StatusUnauthorized, familyRecorder.Code)
		require.Equal(t, "invalid_refresh_token", familyResponse.Error.Code)
	})

	t.Run("concurrent updates return one conflict", func(t *testing.T) {
		payloads := []map[string]any{
			{"priority": "HIGH", "version": task.Version},
			{"priority": "URGENT", "version": task.Version},
		}
		start := make(chan struct{})
		type updateResult struct {
			status int
			body   []byte
		}
		results := make(chan updateResult, len(payloads))
		var wait sync.WaitGroup
		for _, payload := range payloads {
			encoded, err := json.Marshal(payload)
			require.NoError(t, err)
			wait.Add(1)
			go func(body []byte) {
				defer wait.Done()
				<-start
				request := httptest.NewRequest(http.MethodPatch, "/api/v1/tasks/"+task.ID.String(), bytes.NewReader(body))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Authorization", "Bearer "+ownerA.AccessToken)
				recorder := httptest.NewRecorder()
				f.router.ServeHTTP(recorder, request)
				results <- updateResult{status: recorder.Code, body: append([]byte(nil), recorder.Body.Bytes()...)}
			}(encoded)
		}
		close(start)
		wait.Wait()
		close(results)
		actual := make([]int, 0, len(payloads))
		for result := range results {
			actual = append(actual, result.status)
			if result.status == http.StatusConflict {
				var response envelope
				require.NoError(t, json.Unmarshal(result.body, &response))
				require.Equal(t, "version_conflict", response.Error.Code)
			}
		}
		sort.Ints(actual)
		require.Equal(t, []int{http.StatusOK, http.StatusConflict}, actual)
	})
}

func registerUser(t *testing.T, f *httpFixture, email, name string) tokenData {
	t.Helper()
	recorder, response := f.request(t, http.MethodPost, "/api/v1/auth/register", "", map[string]any{"email": email, "name": name, "password": integrationPassword})
	require.Equal(t, http.StatusCreated, recorder.Code, recorder.Body.String())
	return decodeData[tokenData](t, response)
}

func createOrganization(t *testing.T, f *httpFixture, token, name, slug string) organizationData {
	t.Helper()
	recorder, response := f.request(t, http.MethodPost, "/api/v1/organizations", token, map[string]any{"name": name, "slug": slug})
	require.Equal(t, http.StatusCreated, recorder.Code, recorder.Body.String())
	return decodeData[organizationData](t, response)
}

func addMember(t *testing.T, f *httpFixture, token string, organizationID uuid.UUID, email, role string) {
	t.Helper()
	recorder, _ := f.request(t, http.MethodPost, "/api/v1/organizations/"+organizationID.String()+"/members", token, map[string]any{"email": email, "role": role})
	require.Equal(t, http.StatusCreated, recorder.Code, recorder.Body.String())
}

func createProject(t *testing.T, f *httpFixture, token string, organizationID uuid.UUID, name string) projectData {
	t.Helper()
	recorder, response := f.request(t, http.MethodPost, "/api/v1/organizations/"+organizationID.String()+"/projects", token, map[string]any{"name": name, "description": "integration project"})
	require.Equal(t, http.StatusCreated, recorder.Code, recorder.Body.String())
	return decodeData[projectData](t, response)
}

func createTask(t *testing.T, f *httpFixture, token string, projectID uuid.UUID, title string) taskData {
	t.Helper()
	recorder, response := f.request(t, http.MethodPost, "/api/v1/projects/"+projectID.String()+"/tasks", token, map[string]any{"title": title, "priority": "MEDIUM"})
	require.Equal(t, http.StatusCreated, recorder.Code, recorder.Body.String())
	return decodeData[taskData](t, response)
}
