package handler

import (
	"net/http"

	"github.com/example/teamops/backend/internal/modules/tasks/dto"
	"github.com/example/teamops/backend/internal/modules/tasks/model"
	tasksvc "github.com/example/teamops/backend/internal/modules/tasks/service"
	"github.com/example/teamops/backend/internal/shared/errors"
	"github.com/example/teamops/backend/internal/shared/pagination"
	sharedrequest "github.com/example/teamops/backend/internal/shared/request"
	"github.com/example/teamops/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct{ service *tasksvc.Service }

func New(s *tasksvc.Service) *Handler { return &Handler{service: s} }
func parse(c *gin.Context, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		return uuid.Nil, apperror.ErrValidation
	}
	return id, nil
}
func uid(c *gin.Context) uuid.UUID { return c.MustGet("user_id").(uuid.UUID) }
func (h *Handler) Create(c *gin.Context) {
	pid, err := parse(c, "projectId")
	if err != nil {
		response.Error(c, err)
		return
	}
	var req dto.CreateRequest
	if err = sharedrequest.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	t := model.Task{Title: req.Title, Description: req.Description, AssigneeID: req.AssigneeID, Status: req.Status, Priority: req.Priority, DueAt: req.DueAt}
	t, err = h.service.Create(c.Request.Context(), uid(c), pid, t, c.GetString("request_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, dto.FromTask(t))
}
func (h *Handler) List(c *gin.Context) {
	pid, err := parse(c, "projectId")
	if err != nil {
		response.Error(c, err)
		return
	}
	p := pagination.From(c, map[string]bool{"created_at": true, "updated_at": true, "due_at": true, "priority": true, "title": true}, "created_at")
	f := model.Filters{Status: model.Status(c.Query("status")), Priority: model.Priority(c.Query("priority")), Search: c.Query("search")}
	rawAssignee := c.Query("assignee_id")
	if rawAssignee == "" {
		rawAssignee = c.Query("assigneeId")
	}
	if rawAssignee != "" {
		id, e := uuid.Parse(rawAssignee)
		if e != nil {
			response.Error(c, apperror.ErrValidation)
			return
		}
		f.AssigneeID = &id
	}
	items, total, err := h.service.List(c.Request.Context(), uid(c), pid, p, f)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.List(c, dto.FromTasks(items), pagination.NewMeta(p, total))
}
func (h *Handler) Get(c *gin.Context) {
	id, err := parse(c, "taskId")
	if err != nil {
		response.Error(c, err)
		return
	}
	t, err := h.service.Get(c.Request.Context(), uid(c), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, dto.FromTask(t))
}
func (h *Handler) Update(c *gin.Context) {
	id, err := parse(c, "taskId")
	if err != nil {
		response.Error(c, err)
		return
	}
	var req dto.UpdateRequest
	if err = sharedrequest.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	input := tasksvc.UpdateInput{Title: req.Title, Description: req.Description, AssigneeSet: req.AssigneeID.Set, AssigneeID: req.AssigneeID.Value, Status: req.Status, Priority: req.Priority, DueAtSet: req.DueAt.Set, DueAt: req.DueAt.Value, Version: req.Version}
	t, err := h.service.Update(c.Request.Context(), uid(c), id, input, c.GetString("request_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, dto.FromTask(t))
}
func (h *Handler) Delete(c *gin.Context) {
	id, err := parse(c, "taskId")
	if err != nil {
		response.Error(c, err)
		return
	}
	if err = h.service.Delete(c.Request.Context(), uid(c), id, c.GetString("request_id")); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *Handler) Comments(c *gin.Context) {
	id, err := parse(c, "taskId")
	if err != nil {
		response.Error(c, err)
		return
	}
	items, err := h.service.Comments(c.Request.Context(), uid(c), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, dto.FromComments(items))
}
func (h *Handler) AddComment(c *gin.Context) {
	id, err := parse(c, "taskId")
	if err != nil {
		response.Error(c, err)
		return
	}
	var req dto.CommentRequest
	if err = sharedrequest.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	item, err := h.service.AddComment(c.Request.Context(), uid(c), id, req.Body, c.GetString("request_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, dto.FromComment(item))
}
func (h *Handler) Labels(c *gin.Context) {
	id, err := parse(c, "organizationId")
	if err != nil {
		response.Error(c, err)
		return
	}
	items, err := h.service.Labels(c.Request.Context(), uid(c), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, dto.FromLabels(items))
}
func (h *Handler) CreateLabel(c *gin.Context) {
	id, err := parse(c, "organizationId")
	if err != nil {
		response.Error(c, err)
		return
	}
	var req dto.LabelRequest
	if err = sharedrequest.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	item, err := h.service.CreateLabel(c.Request.Context(), uid(c), id, req.Name, req.Color, c.GetString("request_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, dto.FromLabel(item))
}
func (h *Handler) SetLabels(c *gin.Context) {
	id, err := parse(c, "taskId")
	if err != nil {
		response.Error(c, err)
		return
	}
	var req dto.SetLabelsRequest
	if err = sharedrequest.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	if err = h.service.SetLabels(c.Request.Context(), uid(c), id, req.LabelIDs, c.GetString("request_id")); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
