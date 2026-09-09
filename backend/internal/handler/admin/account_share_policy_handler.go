// Package admin provides admin-facing HTTP handlers.
package admin

import (
	"strconv"

	"github.com/your-org/ai-gateway-platform/internal/pkg/response"
	middleware2 "github.com/your-org/ai-gateway-platform/internal/server/middleware"
	"github.com/your-org/ai-gateway-platform/internal/service"

	"github.com/gin-gonic/gin"
)

// AccountSharePolicyHandler handles admin share-policy management.
type AccountSharePolicyHandler struct {
	policyService *service.SharePolicyService
}

// NewAccountSharePolicyHandler creates a new AccountSharePolicyHandler.
func NewAccountSharePolicyHandler(policyService *service.SharePolicyService) *AccountSharePolicyHandler {
	return &AccountSharePolicyHandler{policyService: policyService}
}

// List handles GET /api/v1/admin/share-policies
func (h *AccountSharePolicyHandler) List(c *gin.Context) {
	items, err := h.policyService.List(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

// CreateSharePolicyRequest is the payload for creating a policy.
type CreateSharePolicyRequest struct {
	ScopeType        string   `json:"scope_type"`
	ScopeID          *int64   `json:"scope_id"`
	Platform         *string  `json:"platform"`
	OwnerShareRatio  float64  `json:"owner_share_ratio" binding:"required"`
	InviteShareRatio float64  `json:"invite_share_ratio"`
	Enabled          *bool    `json:"enabled"`
}

// Create handles POST /api/v1/admin/share-policies
func (h *AccountSharePolicyHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req CreateSharePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request payload")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	p := &service.SharePolicy{
		ScopeType:        req.ScopeType,
		ScopeID:          req.ScopeID,
		Platform:         req.Platform,
		OwnerShareRatio:  req.OwnerShareRatio,
		InviteShareRatio: req.InviteShareRatio,
		Enabled:          enabled,
		CreatedByAdminID: &subject.UserID,
	}
	created, err := h.policyService.Create(c.Request.Context(), p)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, created)
}

// Update handles PUT /api/v1/admin/share-policies/:id
func (h *AccountSharePolicyHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid policy id")
		return
	}
	var req CreateSharePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request payload")
		return
	}
	p := &service.SharePolicy{
		ScopeType:        req.ScopeType,
		ScopeID:          req.ScopeID,
		Platform:         req.Platform,
		OwnerShareRatio:  req.OwnerShareRatio,
		InviteShareRatio: req.InviteShareRatio,
		Enabled:          req.Enabled == nil || *req.Enabled,
	}
	updated, err := h.policyService.Update(c.Request.Context(), id, p)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, updated)
}
