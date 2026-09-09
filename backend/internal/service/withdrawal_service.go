// Package service provides business logic and domain services for the application.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"github.com/your-org/ai-gateway-platform/internal/domain"
	infraerrors "github.com/your-org/ai-gateway-platform/internal/pkg/errors"
	"github.com/your-org/ai-gateway-platform/internal/pkg/pagination"
)

// 提现相关哨兵错误。
var (
	ErrWithdrawalNotFound          = infraerrors.NotFound("WITHDRAWAL_NOT_FOUND", "withdrawal not found")
	ErrWithdrawalPendingExists     = infraerrors.Conflict("WITHDRAWAL_PENDING_EXISTS", "owner already has a pending withdrawal")
	ErrWithdrawalInsufficient      = infraerrors.BadRequest("INSUFFICIENT_AVAILABLE_BALANCE", "insufficient available balance")
	ErrWithdrawalInvalidStatus     = infraerrors.Conflict("WITHDRAWAL_INVALID_STATUS", "withdrawal is not in pending status")
	ErrWithdrawalNotOwner          = infraerrors.Forbidden("WITHDRAWAL_NOT_OWNER", "withdrawal does not belong to this owner")
)

// Withdrawal 提现单领域对象。
type Withdrawal struct {
	ID                int64
	OwnerUserID       int64
	Amount            float64
	FeeAmount         float64
	TotalDeducted     float64
	BalanceBefore     float64
	BalanceAfter      float64
	PaymentMethod     string
	PaymentAccount    string
	Status            string
	ProcessedByUserID *int64
	ProcessedAt       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// WithdrawalRepository 定义提现单的数据访问接口。
type WithdrawalRepository interface {
	Create(ctx context.Context, w *Withdrawal) error
	ListByOwner(ctx context.Context, ownerID int64, params pagination.PaginationParams) ([]Withdrawal, *pagination.PaginationResult, error)
	ListAll(ctx context.Context, params pagination.PaginationParams) ([]Withdrawal, *pagination.PaginationResult, error)
	GetByID(ctx context.Context, id int64) (*Withdrawal, error)
	UpdateStatus(ctx context.Context, id int64, status string, processedBy *int64, processedAt *time.Time) error
}

// WithdrawalService 提供号主提现业务能力。
type WithdrawalService struct {
	repo      WithdrawalRepository
	ledgerSvc *OwnerLedgerService
	feeAmount float64 // 提现手续费（默认 0，可配置）
}

// NewWithdrawalService 构造提现服务。
func NewWithdrawalService(repo WithdrawalRepository, ledgerSvc *OwnerLedgerService) *WithdrawalService {
	return &WithdrawalService{repo: repo, ledgerSvc: ledgerSvc, feeAmount: 0}
}

// Submit 发起提现：校验可提现余额 → 写 pending 提现单 → 冻结（withdrawal_hold debit）。
func (s *WithdrawalService) Submit(ctx context.Context, ownerID int64, amount float64, method, account string) (*Withdrawal, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("withdrawal service repo is nil")
	}
	if ownerID <= 0 || amount <= 0 {
		return nil, infraerrors.BadRequest("INVALID_AMOUNT", "amount must be positive")
	}
	if method == "" || account == "" {
		return nil, infraerrors.BadRequest("INVALID_PAYMENT", "payment method and account are required")
	}

	fee := QuantizeUsageBillingAmount(s.feeAmount)
	totalRaw, _ := decimal.NewFromFloat(amount).Add(decimal.NewFromFloat(fee)).Float64()
	total := QuantizeUsageBillingAmount(totalRaw)

	if s.ledgerSvc != nil {
		available, _, err := s.ledgerSvc.Balance(ctx, ownerID)
		if err != nil {
			return nil, err
		}
		if available < total {
			return nil, ErrWithdrawalInsufficient
		}
	}

	w := &Withdrawal{
		OwnerUserID:    ownerID,
		Amount:         QuantizeUsageBillingAmount(amount),
		FeeAmount:      fee,
		TotalDeducted:  total,
		PaymentMethod:  method,
		PaymentAccount: account,
		Status:         domain.WithdrawalStatusPending,
	}
	if err := s.repo.Create(ctx, w); err != nil {
		return nil, err
	}

	if s.ledgerSvc != nil {
		if err := s.ledgerSvc.Debit(ctx, ownerID, total, domain.LedgerEntryTypeWithdrawalHold, "withdrawal", w.ID); err != nil {
			return nil, err
		}
	}
	return w, nil
}

// Approve 审核通过：pending → settled，记录处理人/时间。
func (s *WithdrawalService) Approve(ctx context.Context, adminID, withdrawalID int64) (*Withdrawal, error) {
	w, err := s.get(ctx, withdrawalID)
	if err != nil {
		return nil, err
	}
	if w.Status != domain.WithdrawalStatusPending {
		return nil, ErrWithdrawalInvalidStatus
	}
	now := time.Now().UTC()
	if err := s.repo.UpdateStatus(ctx, withdrawalID, domain.WithdrawalStatusSettled, &adminID, &now); err != nil {
		return nil, err
	}
	w.Status = domain.WithdrawalStatusSettled
	w.ProcessedByUserID = &adminID
	w.ProcessedAt = &now
	return w, nil
}

// Reject 审核拒绝：pending → rejected，解冻（withdrawal_release credit）。
func (s *WithdrawalService) Reject(ctx context.Context, adminID, withdrawalID int64, note string) (*Withdrawal, error) {
	w, err := s.get(ctx, withdrawalID)
	if err != nil {
		return nil, err
	}
	if w.Status != domain.WithdrawalStatusPending {
		return nil, ErrWithdrawalInvalidStatus
	}
	now := time.Now().UTC()
	if err := s.repo.UpdateStatus(ctx, withdrawalID, domain.WithdrawalStatusRejected, &adminID, &now); err != nil {
		return nil, err
	}
	if s.ledgerSvc != nil {
		if err := s.ledgerSvc.Credit(ctx, w.OwnerUserID, w.TotalDeducted, domain.LedgerEntryTypeWithdrawalRelease, "withdrawal", w.ID); err != nil {
			return nil, err
		}
	}
	w.Status = domain.WithdrawalStatusRejected
	w.ProcessedByUserID = &adminID
	w.ProcessedAt = &now
	return w, nil
}

// Cancel 号主取消：pending → cancelled，解冻（withdrawal_release credit）。
func (s *WithdrawalService) Cancel(ctx context.Context, ownerID, withdrawalID int64, reason string) (*Withdrawal, error) {
	w, err := s.get(ctx, withdrawalID)
	if err != nil {
		return nil, err
	}
	if w.OwnerUserID != ownerID {
		return nil, ErrWithdrawalNotOwner
	}
	if w.Status != domain.WithdrawalStatusPending {
		return nil, ErrWithdrawalInvalidStatus
	}
	now := time.Now().UTC()
	if err := s.repo.UpdateStatus(ctx, withdrawalID, domain.WithdrawalStatusCancelled, nil, &now); err != nil {
		return nil, err
	}
	if s.ledgerSvc != nil {
		if err := s.ledgerSvc.Credit(ctx, w.OwnerUserID, w.TotalDeducted, domain.LedgerEntryTypeWithdrawalRelease, "withdrawal", w.ID); err != nil {
			return nil, err
		}
	}
	w.Status = domain.WithdrawalStatusCancelled
	w.ProcessedAt = &now
	return w, nil
}

// List 返回号主提现单列表。
func (s *WithdrawalService) List(ctx context.Context, ownerID int64, params pagination.PaginationParams) ([]Withdrawal, *pagination.PaginationResult, error) {
	if s == nil || s.repo == nil {
		return nil, nil, fmt.Errorf("withdrawal service repo is nil")
	}
	return s.repo.ListByOwner(ctx, ownerID, params)
}

// ListAll 返回全部提现单（管理面）。
func (s *WithdrawalService) ListAll(ctx context.Context, params pagination.PaginationParams) ([]Withdrawal, *pagination.PaginationResult, error) {
	if s == nil || s.repo == nil {
		return nil, nil, fmt.Errorf("withdrawal service repo is nil")
	}
	return s.repo.ListAll(ctx, params)
}

func (s *WithdrawalService) get(ctx context.Context, id int64) (*Withdrawal, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("withdrawal service repo is nil")
	}
	w, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return nil, ErrWithdrawalNotFound
	}
	return w, nil
}

