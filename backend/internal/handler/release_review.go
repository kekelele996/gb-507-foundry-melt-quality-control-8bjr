package handler

import (
	"net/http"
	"strings"

	"github.com/blueship581/foundry-melt-quality-control/backend/internal/dto"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/middleware"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/model"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/service"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/util"
	"github.com/gin-gonic/gin"
)

// ReleaseReviewHandler exposes the 炉次放行合议 workflow: panel listing, the
// pairing/readings/blockers view and the atomic accept/remelt/scrap verdict.
type ReleaseReviewHandler struct {
	service service.ReleaseReviewService
}

func NewReleaseReviewHandler(s service.ReleaseReviewService) *ReleaseReviewHandler {
	return &ReleaseReviewHandler{service: s}
}

func (h *ReleaseReviewHandler) Register(group *gin.RouterGroup) {
	group.GET("/release-panels", h.listHeats)
	group.GET("/release-panels/:heatCode", h.getPanel)
	group.POST("/release-adjudications", middleware.RequireRoles(model.RoleReviewer, model.RoleAdmin), h.adjudicate)
}

func (h *ReleaseReviewHandler) listHeats(c *gin.Context) {
	items, err := h.service.ListPanelHeats(c.Request.Context())
	if err != nil {
		handleError(c, err)
		return
	}
	util.OK(c, items)
}

func (h *ReleaseReviewHandler) getPanel(c *gin.Context) {
	heatCode := strings.TrimSpace(c.Param("heatCode"))
	if heatCode == "" || len(heatCode) > 64 {
		util.Fail(c, http.StatusBadRequest, "invalid_heat_code", "heatCode must be 1-64 characters")
		return
	}
	panel, err := h.service.GetPanel(c.Request.Context(), heatCode)
	if err != nil {
		handleError(c, err)
		return
	}
	util.OK(c, panel)
}

func (h *ReleaseReviewHandler) adjudicate(c *gin.Context) {
	var input dto.ReleaseAdjudicationRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		util.Fail(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := h.service.Adjudicate(c.Request.Context(), input, actorFromContext(c), requestIDFromContext(c))
	if err != nil {
		handleError(c, err)
		return
	}
	util.Created(c, item)
}
