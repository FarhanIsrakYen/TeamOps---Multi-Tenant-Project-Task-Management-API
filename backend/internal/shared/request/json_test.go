package request_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/teamops/backend/internal/middleware"
	sharedrequest "github.com/example/teamops/backend/internal/shared/request"
	"github.com/example/teamops/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type payload struct {
	Name string `json:"name" binding:"required,max=10"`
}

func testRouter(limit int64) *gin.Engine {
	router := gin.New()
	router.Use(middleware.RequestID(), middleware.BodyLimit(limit))
	router.POST("/", func(c *gin.Context) {
		var body payload
		if err := sharedrequest.BindJSON(c, &body); err != nil {
			response.Error(c, err)
			return
		}
		response.OK(c, body)
	})
	return router
}

func TestBindJSONRejectsUnknownFieldsAndTrailingValues(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{"name":"valid","admin":true}`, `{"name":"valid"}{"name":"again"}`} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		testRouter(1024).ServeHTTP(recorder, request)
		require.Equal(t, http.StatusBadRequest, recorder.Code)
		require.Contains(t, recorder.Body.String(), `"code":"validation_error"`)
	}
}

func TestBindJSONEnforcesContentTypeAndStreamingBodyLimit(t *testing.T) {
	t.Parallel()
	t.Run("content type", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"valid"}`))
		testRouter(1024).ServeHTTP(recorder, request)
		require.Equal(t, http.StatusUnsupportedMediaType, recorder.Code)
	})
	t.Run("body limit", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"too large"}`))
		request.Header.Set("Content-Type", "application/json")
		request.ContentLength = -1
		testRouter(8).ServeHTTP(recorder, request)
		require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
		require.Contains(t, recorder.Body.String(), `"code":"request_body_too_large"`)
	})
}
