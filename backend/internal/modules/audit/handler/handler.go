package handler

import (
	"strings"
	"time"

	auditdto "github.com/example/teamops/backend/internal/modules/audit/dto"
	auditmodel "github.com/example/teamops/backend/internal/modules/audit/model"
	auditsvc "github.com/example/teamops/backend/internal/modules/audit/service"
	"github.com/example/teamops/backend/internal/shared/errors"
	"github.com/example/teamops/backend/internal/shared/pagination"
	"github.com/example/teamops/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service *auditsvc.Service
}

func New(service *auditsvc.Service) *Handler { return &Handler{service: service} }
func (h *Handler) List(c *gin.Context) {
	oid, err := uuid.Parse(c.Param("organizationId"))
	if err != nil {
		response.Error(c, apperror.ErrValidation)
		return
	}
	uid := c.MustGet("user_id").(uuid.UUID)
	p := pagination.From(c, map[string]bool{"created_at": true}, "created_at")
	filters, err := filtersFrom(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	items, total, err := h.service.List(c, uid, oid, p, filters)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.List(c, auditdto.FromModels(items), pagination.NewMeta(p, total))
}

func filtersFrom(c *gin.Context) (auditmodel.Filters, error) {
	filters := auditmodel.Filters{
		Action:       c.Query("action"),
		ResourceType: first(c.Query("resource_type"), c.Query("resourceType")),
		ResourceID:   first(c.Query("resource_id"), c.Query("resourceId"), c.Query("resource")),
	}
	actor := first(c.Query("actor_id"), c.Query("actorId"), c.Query("actor_user_id"), c.Query("actorUserId"))
	if actor != "" {
		id, err := uuid.Parse(actor)
		if err != nil {
			return filters, apperror.ErrValidation
		}
		filters.ActorUserID = &id
	}
	from, err := parseTime(c.Query("from"))
	if err != nil {
		return filters, err
	}
	to, err := parseTime(c.Query("to"))
	if err != nil {
		return filters, err
	}
	filters.From, filters.To = from, to
	return filters, nil
}

func parseTime(value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, apperror.New(400, "invalid_date", "audit date filters must use RFC3339")
	}
	return &parsed, nil
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
