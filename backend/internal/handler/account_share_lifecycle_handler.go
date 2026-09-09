// Package handler provides HTTP request handlers for the application.
package handler

import (
	"strconv"

	"github.com/your-org/ai-gateway-platform/internal/pkg/response"
	middleware2 "github.com/your-org/ai-gateway-platform/internal/server/middleware"
	"github.com/your-org/ai-gateway-platform/internal/service"

	"github.com/gin-gonic/gin"
)

// AccountShareLifecycleHandler handles plaza membership lifecycle requests.
type AccountShareLifecycleHandler struct {
	shareModeService *service.AccountShareModeService
}

// NewAccountShareLifecycleHandler creates a new AccountShareLifecycleHandler.
func NewAccountShareLifecycleHandler(shareModeService *service.AccountShareModeService) *AccountShareLifecycleHandler {
	return &AccountShareLifecycleHandler{shareModeService: shareModeService}
}

// EndMembership handles POST /api/v1/share/memberships/:id/end (退座/到期).
func (h *AccountShareLifecycleHandler) EndMembership(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid membership id")
		return
	}
	m, err := h.shareModeService.EndMembership(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, m)
}
