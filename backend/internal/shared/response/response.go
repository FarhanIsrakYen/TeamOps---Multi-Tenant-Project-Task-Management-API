package response

import (
	"net/http"

	"github.com/example/teamops/backend/internal/shared/errors"
	"github.com/gin-gonic/gin"
)

type Envelope struct {
	Data  any        `json:"data,omitempty"`
	Meta  any        `json:"meta,omitempty"`
	Error *ErrorBody `json:"error,omitempty"`
}
type ErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

func OK(c *gin.Context, data any)         { c.JSON(http.StatusOK, Envelope{Data: data}) }
func Created(c *gin.Context, data any)    { c.JSON(http.StatusCreated, Envelope{Data: data}) }
func List(c *gin.Context, data, meta any) { c.JSON(http.StatusOK, Envelope{Data: data, Meta: meta}) }
func Error(c *gin.Context, err error) {
	ae := apperror.As(err)
	c.JSON(ae.Status, Envelope{Error: &ErrorBody{Code: ae.Code, Message: ae.Message, RequestID: c.GetString("request_id")}})
}
