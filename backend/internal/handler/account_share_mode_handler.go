// Package handler provides HTTP request handlers for the application.
package handler

import (
	"strconv"

	"github.com/your-org/ai-gateway-platform/internal/pkg/response"
	middleware2 "github.com/your-org/ai-gateway-platform/internal/server/middleware"
	"github.com/your-org/ai-gateway-platform/internal/service"

	"github.com/gin-gonic/gin"
)

// AccountShareModeHandler handles plaza room CRUD, join and queue requests.
type AccountShareModeHandler struct {
	shareModeService *service.AccountShareModeService
}

// NewAccountShareModeHandler creates a new AccountShareModeHandler.
func NewAccountShareModeHandler(shareModeService *service.AccountShareModeService) *AccountShareModeHandler {
	return &AccountShareModeHandler{shareModeService: shareModeService}
}

// CreateRoomRequest is the payload for creating a plaza room.
type CreateRoomRequest struct {
	AccountID int64  `json:"account_id" binding:"required"`
	Name      string `json:"name" binding:"required"`
	SeatLimit int    `json:"seat_limit"`
	QueueMax  int    `json:"queue_max"`
}

// CreateRoom handles POST /api/v1/share/rooms (owner creates a room for their hosted account).
func (h *AccountShareModeHandler) CreateRoom(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request payload")
		return
	}
	room, err := h.shareModeService.CreateRoom(c.Request.Context(), subject.UserID, req.AccountID, req.Name, req.SeatLimit, req.QueueMax)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, room)
}

// UpdateRoomRequest is the payload for updating a room.
type UpdateRoomRequest struct {
	Name      string `json:"name"`
	SeatLimit int    `json:"seat_limit"`
	QueueMax  int    `json:"queue_max"`
}

// UpdateRoom handles PUT /api/v1/share/rooms/:id
func (h *AccountShareModeHandler) UpdateRoom(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid room id")
		return
	}
	var req UpdateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request payload")
		return
	}
	room, err := h.shareModeService.UpdateRoom(c.Request.Context(), subject.UserID, id, req.Name, req.SeatLimit, req.QueueMax)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, room)
}

// JoinRequest is the payload for joining a room.
type JoinRequest struct {
	APIKeyID *int64 `json:"api_key_id"`
}

// Join handles POST /api/v1/share/rooms/:id/join
func (h *AccountShareModeHandler) Join(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid room id")
		return
	}
	var req JoinRequest
	_ = c.ShouldBindJSON(&req)
	m, err := h.shareModeService.Join(c.Request.Context(), subject.UserID, req.APIKeyID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, m)
}

// Queue handles POST /api/v1/share/rooms/:id/queue
func (h *AccountShareModeHandler) Queue(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid room id")
		return
	}
	m, err := h.shareModeService.Queue(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, m)
}
