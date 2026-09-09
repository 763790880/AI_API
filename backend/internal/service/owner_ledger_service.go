// Package service provides business logic and domain services for the application.
package service

import (
	"context"
	"errors"
	"time"

	"github.com/shopspring/decimal"
	"github.com/your-org/ai-gateway-platform/internal/domain"
	"github.com/your-org/ai-gateway-platform/internal/pkg/pagination"
)

// OwnerLedgerEntry 描述一条待写入的号主收益账本流水。
type OwnerLedgerEntry struct {
	OwnerUserID   int64
	Direction     string
	Amount        float64
	BalanceBefore float64
	BalanceAfter  float64
	EntryType     string
	RefType       string
	RefID         int64
	Metadata      map[string]any
}

// OwnerLedger 收益账本流水展示对象（用于查询）。
type OwnerLedger struct {
	ID            int64
	OwnerUserID   int64
	Direction     string
	Amount        float64
	BalanceBefore float64
	BalanceAfter  float64
	EntryType     string
	RefType       *string
	RefID         *int64
	CreatedAt     time.Time
}

// OwnerLedgerRepository 定义号主收益账本的数据访问接口。
type OwnerLedgerRepository interface {
	// Credit 写入一条账本流水（方向由 entry.Direction 决定）。
	// 重复的 (ref_type, ref_id) 由部分唯一索引拦截并静默跳过（幂等）。
	Credit(ctx context.Context, entry OwnerLedgerEntry) error
	// Balance 返回号主收益余额双值：available（可提现）、pending（待结算）。
	Balance(ctx context.Context, ownerID int64) (available, pending float64, err error)
	// ListByOwner 返回号主收益流水（供后台查询）。
	ListByOwner(ctx context.Context, ownerID int64, params pagination.PaginationParams) ([]OwnerLedger, *pagination.PaginationResult, error)
}

// OwnerLedgerService 提供号主收益账本的业务操作。
type OwnerLedgerService struct {
	repo OwnerLedgerRepository
}

// NewOwnerLedgerService 构造收益账本服务。
func NewOwnerLedgerService(repo OwnerLedgerRepository) *OwnerLedgerService {
	return &OwnerLedgerService{repo: repo}
}

// Credit 记一笔入账（direction=credit），并计算 balance_before/balance_after。
func (s *OwnerLedgerService) Credit(ctx context.Context, ownerID int64, amount float64, entryType, refType string, refID int64) error {
	return s.append(ctx, ownerID, amount, entryType, refType, refID, domain.LedgerDirectionCredit)
}

// Debit 记一笔出账（direction=debit），并计算 balance_before/balance_after。
func (s *OwnerLedgerService) Debit(ctx context.Context, ownerID int64, amount float64, entryType, refType string, refID int64) error {
	return s.append(ctx, ownerID, amount, entryType, refType, refID, domain.LedgerDirectionDebit)
}

// Balance 返回号主收益余额双值：available（可提现）、pending（待结算）。
func (s *OwnerLedgerService) Balance(ctx context.Context, ownerID int64) (available, pending float64, err error) {
	if s == nil || s.repo == nil {
		return 0, 0, errors.New("owner ledger service repo is nil")
	}
	return s.repo.Balance(ctx, ownerID)
}

// List 返回号主收益流水。
func (s *OwnerLedgerService) List(ctx context.Context, ownerID int64, params pagination.PaginationParams) ([]OwnerLedger, *pagination.PaginationResult, error) {
	if s == nil || s.repo == nil {
		return []OwnerLedger{}, &pagination.PaginationResult{}, nil
	}
	return s.repo.ListByOwner(ctx, ownerID, params)
}

// append 计算快照并写入一条账本流水（金额一律 decimal，落库前 8 位量化）。
//
// balance_before/balance_after 记录「经济总头寸」available+pending 的变动前后值。
func (s *OwnerLedgerService) append(ctx context.Context, ownerID int64, amount float64, entryType, refType string, refID int64, direction string) error {
	if s == nil || s.repo == nil {
		return errors.New("owner ledger service repo is nil")
	}
	if ownerID <= 0 || amount <= 0 {
		return nil
	}

	available, pending, err := s.repo.Balance(ctx, ownerID)
	if err != nil {
		return err
	}

	total := decimal.NewFromFloat(available).Add(decimal.NewFromFloat(pending))
	amt := decimal.NewFromFloat(amount)
	var after decimal.Decimal
	if direction == domain.LedgerDirectionCredit {
		after = total.Add(amt)
	} else {
		after = total.Sub(amt)
	}
	beforeF, _ := total.Round(UsageBillingMonetaryScale).Float64()
	afterF, _ := after.Round(UsageBillingMonetaryScale).Float64()

	entry := OwnerLedgerEntry{
		OwnerUserID:   ownerID,
		Direction:     direction,
		Amount:        QuantizeUsageBillingAmount(amount),
		BalanceBefore: beforeF,
		BalanceAfter:  afterF,
		EntryType:     entryType,
		RefType:       refType,
		RefID:         refID,
	}
	return s.repo.Credit(ctx, entry)
}
