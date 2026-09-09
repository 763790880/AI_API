// Package service provides business logic and domain services for the application.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/your-org/ai-gateway-platform/internal/domain"
	infraerrors "github.com/your-org/ai-gateway-platform/internal/pkg/errors"
)

// 广场房间/会员相关哨兵错误。
var (
	ErrRoomNotFound         = infraerrors.NotFound("ROOM_NOT_FOUND", "share room not found")
	ErrRoomAccountConflict  = infraerrors.Conflict("ROOM_ACCOUNT_CONFLICT", "account already has a share room")
	ErrRoomVersionConflict  = infraerrors.Conflict("ROOM_VERSION_CONFLICT", "share room was modified concurrently, retry")
	ErrRoomFull             = infraerrors.Conflict("ROOM_FULL", "share room is full")
	ErrRoomQueueFull        = infraerrors.Conflict("ROOM_QUEUE_FULL", "share room queue is full")
	ErrMembershipNotFound   = infraerrors.NotFound("MEMBERSHIP_NOT_FOUND", "share membership not found")
	ErrMembershipNotOwner   = infraerrors.Forbidden("MEMBERSHIP_NOT_OWNER", "operation not allowed for this actor")
)

// AccountShareRoom 账号广场房间领域对象。
type AccountShareRoom struct {
	ID                 int64
	AccountID          int64
	OwnerUserID        int64
	Name               string
	Status             string
	SeatLimit          int
	RateMultiplier     float64
	AllowedModels      []string
	PerUserConcurrency int
	QueueMax           int
	Version            int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// AccountShareMembership 账号广场会员领域对象。
type AccountShareMembership struct {
	ID             int64
	RoomID         int64
	AccountID      int64
	ConsumerUserID int64
	APIKeyID       *int64
	Status         string
	JoinedAt       time.Time
	EndedAt        *time.Time
	QueueExpiresAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// AccountShareRoomRepository 定义广场房间的数据访问接口。
type AccountShareRoomRepository interface {
	Create(ctx context.Context, r *AccountShareRoom) error
	GetByID(ctx context.Context, id int64) (*AccountShareRoom, error)
	GetByAccountID(ctx context.Context, accountID int64) (*AccountShareRoom, error)
	// Update 带 version 乐观锁更新房间（version 自增）。0 行受影响视为冲突。
	Update(ctx context.Context, r *AccountShareRoom) error
	ListActive(ctx context.Context) ([]AccountShareRoom, error)
}

// AccountShareMembershipRepository 定义广场会员的数据访问接口。
type AccountShareMembershipRepository interface {
	Create(ctx context.Context, m *AccountShareMembership) error
	GetByID(ctx context.Context, id int64) (*AccountShareMembership, error)
	UpdateStatus(ctx context.Context, id int64, status string, endedAt, queueExpiresAt *time.Time) error
	CountActiveSeats(ctx context.Context, roomID int64) (int, error)
	CountQueued(ctx context.Context, roomID int64) (int, error)
	ListByRoom(ctx context.Context, roomID int64) ([]AccountShareMembership, error)
}

// AccountShareModeService 提供账号广场房间与会员业务能力。
type AccountShareModeService struct {
	roomRepo       AccountShareRoomRepository
	membershipRepo AccountShareMembershipRepository
	rc             *redis.Client
}

// NewAccountShareModeService 构造广场服务。
func NewAccountShareModeService(roomRepo AccountShareRoomRepository, membershipRepo AccountShareMembershipRepository, rc *redis.Client) *AccountShareModeService {
	return &AccountShareModeService{roomRepo: roomRepo, membershipRepo: membershipRepo, rc: rc}
}

// CreateRoom 号主为自己托管的账号创建广场房间（account_id 唯一）。
func (s *AccountShareModeService) CreateRoom(ctx context.Context, ownerID, accountID int64, name string, seatLimit, queueMax int) (*AccountShareRoom, error) {
	if s == nil || s.roomRepo == nil {
		return nil, fmt.Errorf("account share mode service room repo is nil")
	}
	if existing, err := s.roomRepo.GetByAccountID(ctx, accountID); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, ErrRoomAccountConflict
	}
	if seatLimit <= 0 {
		seatLimit = 5
	}
	if queueMax <= 0 {
		queueMax = 20
	}
	room := &AccountShareRoom{
		AccountID:     accountID,
		OwnerUserID:   ownerID,
		Name:          name,
		Status:        domain.RoomStatusActive,
		SeatLimit:     seatLimit,
		RateMultiplier: 1,
		QueueMax:      queueMax,
		Version:       0,
	}
	if err := s.roomRepo.Create(ctx, room); err != nil {
		return nil, err
	}
	return room, nil
}

// UpdateRoom 号主更新房间配置（version 乐观锁）。
func (s *AccountShareModeService) UpdateRoom(ctx context.Context, ownerID, roomID int64, name string, seatLimit, queueMax int) (*AccountShareRoom, error) {
	if s == nil || s.roomRepo == nil {
		return nil, fmt.Errorf("account share mode service room repo is nil")
	}
	room, err := s.roomRepo.GetByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, ErrRoomNotFound
	}
	if room.OwnerUserID != ownerID {
		return nil, ErrMembershipNotOwner
	}
	if name != "" {
		room.Name = name
	}
	if seatLimit > 0 {
		room.SeatLimit = seatLimit
	}
	if queueMax > 0 {
		room.QueueMax = queueMax
	}
	if err := s.roomRepo.Update(ctx, room); err != nil {
		return nil, err
	}
	return room, nil
}

// Join 消费者加入房间（占用一个座位，受 seat_limit 约束）。
func (s *AccountShareModeService) Join(ctx context.Context, consumerUserID int64, apiKeyID *int64, roomID int64) (*AccountShareMembership, error) {
	if s == nil || s.roomRepo == nil || s.membershipRepo == nil {
		return nil, fmt.Errorf("account share mode service repo is nil")
	}
	room, err := s.roomRepo.GetByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, ErrRoomNotFound
	}

	// 座位计数：Redis 原子 INCR 做并发闸，DB CountActiveSeats 做持久化核对。
	if s.rc != nil {
		key := plazaSeatKey(roomID)
		n, incrErr := s.rc.Incr(ctx, key).Result()
		if incrErr == nil {
			if n > int64(room.SeatLimit) {
				_, _ = s.rc.Decr(ctx, key).Result()
				return nil, ErrRoomFull
			}
			defer func() {
				_, _ = s.rc.Expire(ctx, key, 2*time.Hour).Result()
			}()
		}
	}
	if active, err := s.membershipRepo.CountActiveSeats(ctx, roomID); err != nil {
		return nil, err
	} else if active >= room.SeatLimit {
		return nil, ErrRoomFull
	}

	m := &AccountShareMembership{
		RoomID:         roomID,
		AccountID:      room.AccountID,
		ConsumerUserID: consumerUserID,
		APIKeyID:       apiKeyID,
		Status:         domain.MembershipStatusActive,
		JoinedAt:       time.Now().UTC(),
	}
	if err := s.membershipRepo.Create(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Queue 满员时消费者进入排队（受 queue_max 约束，默认 2h 过期）。
func (s *AccountShareModeService) Queue(ctx context.Context, consumerUserID int64, roomID int64) (*AccountShareMembership, error) {
	if s == nil || s.roomRepo == nil || s.membershipRepo == nil {
		return nil, fmt.Errorf("account share mode service repo is nil")
	}
	room, err := s.roomRepo.GetByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, ErrRoomNotFound
	}
	if queued, err := s.membershipRepo.CountQueued(ctx, roomID); err != nil {
		return nil, err
	} else if queued >= room.QueueMax {
		return nil, ErrRoomQueueFull
	}

	exp := time.Now().UTC().Add(2 * time.Hour)
	m := &AccountShareMembership{
		RoomID:         roomID,
		AccountID:      room.AccountID,
		ConsumerUserID: consumerUserID,
		Status:         domain.MembershipStatusQueued,
		JoinedAt:       time.Now().UTC(),
		QueueExpiresAt: &exp,
	}
	if err := s.membershipRepo.Create(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// EndMembership 退座/到期：active/queued → ended，并释放座位计数。
func (s *AccountShareModeService) EndMembership(ctx context.Context, actorID, membershipID int64) (*AccountShareMembership, error) {
	if s == nil || s.membershipRepo == nil {
		return nil, fmt.Errorf("account share mode service membership repo is nil")
	}
	m, err := s.membershipRepo.GetByID(ctx, membershipID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrMembershipNotFound
	}
	if m.ConsumerUserID != actorID {
		return nil, ErrMembershipNotOwner
	}
	now := time.Now().UTC()
	if err := s.membershipRepo.UpdateStatus(ctx, membershipID, domain.MembershipStatusEnded, &now, nil); err != nil {
		return nil, err
	}
	if m.Status == domain.MembershipStatusActive && s.rc != nil {
		_, _ = s.rc.Decr(ctx, plazaSeatKey(m.RoomID)).Result()
	}
	m.Status = domain.MembershipStatusEnded
	m.EndedAt = &now
	return m, nil
}

func plazaSeatKey(roomID int64) string {
	return fmt.Sprintf("plaza:seat:%d", roomID)
}
