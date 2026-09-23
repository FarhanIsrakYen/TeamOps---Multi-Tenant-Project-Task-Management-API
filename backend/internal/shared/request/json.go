package request

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	apperror "github.com/example/teamops/backend/internal/shared/errors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// BindJSON decodes exactly one JSON object, rejects unknown fields, and applies
// Gin's validator. It returns only client-safe application errors.
func BindJSON(c *gin.Context, dst any) error {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return apperror.New(http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
	}

	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(dst); err != nil {
		return decodeError(err)
	}
	if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperror.ErrValidation
	}
	if binding.Validator != nil {
		if err = binding.Validator.ValidateStruct(dst); err != nil {
			return apperror.ErrValidation
		}
	}
	return nil
}

func decodeError(err error) error {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		return apperror.New(http.StatusRequestEntityTooLarge, "request_body_too_large", "request body exceeds the allowed size")
	}
	return apperror.ErrValidation
}
