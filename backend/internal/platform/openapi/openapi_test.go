package openapi

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

func loadDocument(t *testing.T) *openapi3.T {
	t.Helper()
	document, err := openapi3.NewLoader().LoadFromData(Document)
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))
	return document
}

func TestDocumentIsValidOpenAPI(t *testing.T) {
	t.Parallel()
	document := loadDocument(t)
	require.Equal(t, "3.1.0", document.OpenAPI)
}

func TestEveryRegisteredRESTRouteHasAnOperation(t *testing.T) {
	t.Parallel()
	document := loadDocument(t)

	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	appSource, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "app", "app.go"))
	require.NoError(t, err)

	routePattern := regexp.MustCompile(`(?m)(authRoutes|secured)\.(GET|POST|PATCH|PUT|DELETE)\("([^"]+)"`)
	parameterPattern := regexp.MustCompile(`:([A-Za-z][A-Za-z0-9]*)`)
	matches := routePattern.FindAllStringSubmatch(string(appSource), -1)
	require.NotEmpty(t, matches)

	for _, match := range matches {
		prefix := "/api/v1"
		if match[1] == "authRoutes" {
			prefix += "/auth"
		}
		path := parameterPattern.ReplaceAllString(prefix+match[3], `{$1}`)
		operation := document.Paths.Find(path)
		require.NotNilf(t, operation, "%s is missing from the OpenAPI paths", path)
		require.NotNilf(t, operation.GetOperation(match[2]), "%s %s is missing from the OpenAPI paths", match[2], path)
	}

	operationalRoutes := map[string]string{
		"/health":       http.MethodGet,
		"/ready":        http.MethodGet,
		"/health/live":  http.MethodGet,
		"/health/ready": http.MethodGet,
		"/metrics":      http.MethodGet,
		"/openapi.yaml": http.MethodGet,
	}
	for path, method := range operationalRoutes {
		pathItem := document.Paths.Find(path)
		require.NotNilf(t, pathItem, "%s is missing from the OpenAPI paths", path)
		require.NotNilf(t, pathItem.GetOperation(method), "%s %s is missing from the OpenAPI paths", method, path)
	}
}

func TestRequiredRequestExamplesArePresent(t *testing.T) {
	t.Parallel()
	document := loadDocument(t)

	exampleOperations := map[string]string{
		"/api/v1/auth/register":                           http.MethodPost,
		"/api/v1/auth/login":                              http.MethodPost,
		"/api/v1/auth/refresh":                            http.MethodPost,
		"/api/v1/organizations":                           http.MethodPost,
		"/api/v1/organizations/{organizationId}/members":  http.MethodPost,
		"/api/v1/organizations/{organizationId}/projects": http.MethodPost,
		"/api/v1/projects/{projectId}/tasks":              http.MethodPost,
		"/api/v1/tasks/{taskId}":                          http.MethodPatch,
	}
	for path, method := range exampleOperations {
		operation := document.Paths.Find(path).GetOperation(method)
		require.NotNilf(t, operation.RequestBody, "%s %s has no request body", method, path)
		mediaType := operation.RequestBody.Value.Content.Get("application/json")
		require.NotNilf(t, mediaType, "%s %s has no JSON media type", method, path)
		require.NotNilf(t, mediaType.Example, "%s %s has no request example", method, path)
	}
}

func TestVersionedOperationsDocumentMiddlewareErrors(t *testing.T) {
	t.Parallel()
	document := loadDocument(t)

	for path, pathItem := range document.Paths.Map() {
		if !strings.HasPrefix(path, "/api/v1/") {
			continue
		}
		for method, operation := range pathItem.Operations() {
			require.NotNilf(t, operation.Responses.Value("429"), "%s %s must document rate limiting", method, path)
			if operation.RequestBody == nil {
				continue
			}
			require.NotNilf(t, operation.Responses.Value("413"), "%s %s must document the body-size limit", method, path)
			require.NotNilf(t, operation.Responses.Value("415"), "%s %s must document the JSON content-type requirement", method, path)
		}
	}
}

func TestSwaggerUIUsesEmbeddedContract(t *testing.T) {
	t.Parallel()
	html := string(SwaggerUI)
	require.Contains(t, html, `url: "/openapi.yaml"`)
	require.Contains(t, html, "swagger-ui-dist@5.33.0")
	require.False(t, strings.Contains(strings.ToLower(html), "persistauthorization: true"))
}
