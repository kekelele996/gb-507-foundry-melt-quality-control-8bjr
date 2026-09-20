package handler

import (
	"net/http"

	"github.com/blueship581/foundry-melt-quality-control/backend/internal/dto"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/middleware"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/model"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/service"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/util"
	"github.com/gin-gonic/gin"
)

type ReleaseReviewHandler struct {
	service service.ReleaseReviewService
}

func NewReleaseReviewHandler(s service.ReleaseReviewService) *ReleaseReviewHandler {
	return &ReleaseReviewHandler{service: s}
}

func (h *ReleaseReviewHandler) Register(group *gin.RouterGroup) {
	resource := group.Group("/release-reviews")
	resource.GET("", h.list)
	resource.GET("/:id", h.get)
	resource.POST("", middleware.RequireRoles(model.RoleReviewer, model.RoleAdmin), h.open)
	resource.POST("/:id/judge", middleware.RequireRoles(model.RoleReviewer, model.RoleAdmin), h.judge)
	resource.DELETE("/:id", middleware.RequireRoles(model.RoleAdmin), h.remove)
	group.GET("/release-board", h.board)
}

func (h *ReleaseReviewHandler) list(c *gin.Context) {
	result, err := h.service.List(c.Request.Context(), bindPage(c))
	if err != nil {
		handleError(c, err)
		return
	}
	util.Page(c, result.Items, result.Page, result.PageSize, result.Total)
}

func (h *ReleaseReviewHandler) board(c *gin.Context) {
	views, err := h.service.PairingBoard(c.Request.Context())
	if err != nil {
		handleError(c, err)
		return
	}
	util.OK(c, views)
}

func (h *ReleaseReviewHandler) get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	item, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *ReleaseReviewHandler) open(c *gin.Context) {
	var input dto.CreateReleaseReview
	if err := c.ShouldBindJSON(&input); err != nil {
		util.Fail(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := h.service.Open(c.Request.Context(), input, actorFromContext(c), requestIDFromContext(c))
	if err != nil {
		handleError(c, err)
		return
	}
	util.Created(c, item)
}

func (h *ReleaseReviewHandler) judge(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var input dto.JudgeReleaseReview
	if err := c.ShouldBindJSON(&input); err != nil {
		util.Fail(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := h.service.Judge(c.Request.Context(), id, input, actorFromContext(c), requestIDFromContext(c))
	if err != nil {
		handleError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *ReleaseReviewHandler) remove(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.service.DeleteOpen(c.Request.Context(), id, actorFromContext(c), requestIDFromContext(c)); err != nil {
		handleError(c, err)
		return
	}
	util.NoContent(c)
}
