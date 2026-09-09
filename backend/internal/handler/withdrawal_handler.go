// Package handler provides HTTP request handlers for the application.
package handler

import (
	"strconv"

	"github.com/your-org/ai-gateway-platform/internal/pkg/pagination"
	"github.com/your-org/ai-gateway-platform/internal/pkg/response"
	middleware2 "github.com/your-org/ai-gateway-platform/internal/server/middleware"
	"github.com/your-org/ai-gateway-platform/internal/service"

	"github.com/gin-gonic/gin"
)

// WithdrawalHandler handles owner-facing withdrawal requests.
type WithdrawalHandler struct {
	withdrawalService *service.WithdrawalService
}

// NewWithdrawalHandler creates a new WithdrawalHandler.
func NewWithdrawalHandler(withdrawalService *service.WithdrawalService) *WithdrawalHandler {
	return &WithdrawalHandler{withdrawalService: withdrawalService}
}

// SubmitWithdrawalRequest is the payload for submitting a withdrawal.
type SubmitWithdrawalRequest struct {
	Amount         float64 `json:"amount" binding:"required"`
	PaymentMethod  string  `json:"payment_method" binding:"required"`
	PaymentAccount string  `json:"payment_account" binding:"required"`
}

// Submit handles POST /api/v1/withdrawals
func (h *WithdrawalHandler) Submit(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req SubmitWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request payload")
		return
	}
	w, err := h.withdrawalService.Submit(c.Request.Context(), subject.UserID, req.Amount, req.PaymentMethod, req.PaymentAccount)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, w)
}

// List handles GET /api/v1/withdrawals
func (h *WithdrawalHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: "created_at", SortOrder: "desc"}
	items, result, err := h.withdrawalService.List(c.Request.Context(), subject.UserID, params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, result.Total, page, pageSize)
}

// Cancel handles POST /api/v1/withdrawals/:id/cancel
func (h *WithdrawalHandler) Cancel(c *gin.Context) {
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
	w, err := h.withdrawalService.Cancel(c.Request.Context(), subject.UserID, id, "owner cancelled")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, w)
}
