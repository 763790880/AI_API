// Package admin provides admin-facing HTTP handlers.
package admin

import (
	"strconv"

	"github.com/your-org/ai-gateway-platform/internal/pkg/pagination"
	"github.com/your-org/ai-gateway-platform/internal/pkg/response"
	middleware2 "github.com/your-org/ai-gateway-platform/internal/server/middleware"
	"github.com/your-org/ai-gateway-platform/internal/service"

	"github.com/gin-gonic/gin"
)

// WithdrawalHandler handles admin withdrawal review.
type WithdrawalHandler struct {
	withdrawalService *service.WithdrawalService
}

// NewWithdrawalHandler creates a new admin WithdrawalHandler.
func NewWithdrawalHandler(withdrawalService *service.WithdrawalService) *WithdrawalHandler {
	return &WithdrawalHandler{withdrawalService: withdrawalService}
}

// List handles GET /api/v1/admin/withdrawals
func (h *WithdrawalHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: "created_at", SortOrder: "desc"}
	items, result, err := h.withdrawalService.ListAll(c.Request.Context(), params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, result.Total, page, pageSize)
}

// Approve handles POST /api/v1/admin/withdrawals/:id/approve
func (h *WithdrawalHandler) Approve(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid withdrawal id")
		return
	}
	w, err := h.withdrawalService.Approve(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, w)
}

// Reject handles POST /api/v1/admin/withdrawals/:id/reject
func (h *WithdrawalHandler) Reject(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid withdrawal id")
		return
	}
	w, err := h.withdrawalService.Reject(c.Request.Context(), subject.UserID, id, "rejected by admin")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, w)
}
