// Package admin provides admin-facing HTTP handlers.
package admin

import (
	"strconv"
	"time"

	"github.com/your-org/ai-gateway-platform/internal/pkg/pagination"
	"github.com/your-org/ai-gateway-platform/internal/pkg/response"
	"github.com/your-org/ai-gateway-platform/internal/service"

	"github.com/gin-gonic/gin"
)

// RevenueHandler handles admin settlement, revenue and ledger queries.
type RevenueHandler struct {
	settlementService *service.SettlementService
	ledgerService     *service.OwnerLedgerService
}

// NewRevenueHandler creates a new RevenueHandler.
func NewRevenueHandler(settlementService *service.SettlementService, ledgerService *service.OwnerLedgerService) *RevenueHandler {
	return &RevenueHandler{settlementService: settlementService, ledgerService: ledgerService}
}

// ListSettlements handles GET /api/v1/admin/settlements
func (h *RevenueHandler) ListSettlements(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: "created_at", SortOrder: "desc"}
	items, result, err := h.settlementService.ListAll(c.Request.Context(), params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, result.Total, page, pageSize)
}

// Settle handles POST /api/v1/admin/settlements/:id/settle
func (h *RevenueHandler) Settle(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid settlement id")
		return
	}
	st, err := h.settlementService.Settle(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, st)
}

// ExportSettlements handles GET /api/v1/admin/settlements/export (CSV)
func (h *RevenueHandler) ExportSettlements(c *gin.Context) {
	start, _ := time.Parse(time.RFC3339, c.Query("period_start"))
	end, _ := time.Parse(time.RFC3339, c.Query("period_end"))
	data, err := h.settlementService.ExportCSV(c.Request.Context(), start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=settlements.csv")
	_, _ = c.Writer.Write(data)
}

// ListLedger handles GET /api/v1/admin/ledger/:ownerID
func (h *RevenueHandler) ListLedger(c *gin.Context) {
	ownerID, err := strconv.ParseInt(c.Param("ownerID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid owner id")
		return
	}
	available, pending, err := h.ledgerService.Balance(c.Request.Context(), ownerID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: "created_at", SortOrder: "desc"}
	items, result, err := h.ledgerService.List(c.Request.Context(), ownerID, params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{
		"available": available,
		"pending":   pending,
		"items":     items,
		"total":     result.Total,
		"page":      page,
		"page_size": pageSize,
	})
}
