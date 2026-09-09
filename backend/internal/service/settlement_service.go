// Package service provides business logic and domain services for the application.
package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"github.com/your-org/ai-gateway-platform/internal/domain"
	"github.com/your-org/ai-gateway-platform/internal/pkg/pagination"
)

// Settlement 结算单领域对象。
type Settlement struct {
	ID           int64
	OwnerUserID  int64
	SettlementNo string
	PeriodStart  time.Time
	PeriodEnd    time.Time
	GrossAmount  float64
	FeeAmount    float64
	NetAmount    float64
	Status       string
	SettledAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// OwnerShareCreditAgg 号主在结算周期内的 share_credit 汇总。
type OwnerShareCreditAgg struct {
	OwnerUserID int64
	Total       float64
}

// SettlementRepository 定义结算单的数据访问接口。
type SettlementRepository interface {
	Create(ctx context.Context, s *Settlement) error
	ListByOwner(ctx context.Context, ownerID int64, params pagination.PaginationParams) ([]Settlement, *pagination.PaginationResult, error)
	GetByID(ctx context.Context, id int64) (*Settlement, error)
	UpdateStatus(ctx context.Context, id int64, status string, settledAt *time.Time) error
	ListByPeriod(ctx context.Context, start, end time.Time) ([]Settlement, error)
	ListAll(ctx context.Context, params pagination.PaginationParams) ([]Settlement, *pagination.PaginationResult, error)
	SumShareCreditByOwner(ctx context.Context, start, end time.Time) ([]OwnerShareCreditAgg, error)
}

// SettlementService 提供结算与分账入账能力。
type SettlementService struct {
	policySvc *SharePolicyService
	ledgerSvc *OwnerLedgerService
	repo      SettlementRepository
}

// NewSettlementService 构造结算服务。
func NewSettlementService(policySvc *SharePolicyService, ledgerSvc *OwnerLedgerService, repo SettlementRepository) *SettlementService {
	return &SettlementService{policySvc: policySvc, ledgerSvc: ledgerSvc, repo: repo}
}

// RecordShareCredit 按生效分成比例将消费者实付金额计入号主收益账本（T03）。
func (s *SettlementService) RecordShareCredit(ctx context.Context, ownerID, usageLogID int64, consumerCharge float64) error {
	if s == nil || ownerID <= 0 || usageLogID <= 0 || consumerCharge <= 0 {
		return nil
	}
	if s.policySvc == nil || s.ledgerSvc == nil {
		return nil
	}

	ratio, err := s.policySvc.ResolveEnabledRatio(ctx)
	if err != nil {
		return err
	}

	ownerCredit := decimal.NewFromFloat(consumerCharge).Mul(decimal.NewFromFloat(ratio))
	raw, _ := ownerCredit.Float64()
	ownerCreditF := QuantizeUsageBillingAmount(raw)
	if ownerCreditF <= 0 {
		return nil
	}

	return s.ledgerSvc.Credit(ctx, ownerID, ownerCreditF, domain.LedgerEntryTypeShareCredit, "usage_log", usageLogID)
}

// GenerateSettlements 汇总每个号主在周期内的 share_credit，生成 pending 结算单。
func (s *SettlementService) GenerateSettlements(ctx context.Context, periodStart, periodEnd time.Time) ([]Settlement, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("settlement service repo is nil")
	}
	aggs, err := s.repo.SumShareCreditByOwner(ctx, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}

	out := make([]Settlement, 0, len(aggs))
	for _, agg := range aggs {
		if agg.Total <= 0 {
			continue
		}
		st := Settlement{
			OwnerUserID:  agg.OwnerUserID,
			SettlementNo: fmt.Sprintf("SM-%d-%d", periodStart.Unix(), agg.OwnerUserID),
			PeriodStart:  periodStart,
			PeriodEnd:    periodEnd,
			GrossAmount:  QuantizeUsageBillingAmount(agg.Total),
			FeeAmount:    0,
			NetAmount:    QuantizeUsageBillingAmount(agg.Total),
			Status:       domain.SettlementStatusPending,
		}
		if err := s.repo.Create(ctx, &st); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, nil
}

// Settle 将 pending 结算单置为 settled，并向号主写 settlement_payout 流水（pending → available）。
func (s *SettlementService) Settle(ctx context.Context, settlementID int64) (*Settlement, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("settlement service repo is nil")
	}
	st, err := s.repo.GetByID(ctx, settlementID)
	if err != nil {
		return nil, err
	}
	if st.Status != domain.SettlementStatusPending {
		return nil, fmt.Errorf("settlement %d is not pending (status=%s)", settlementID, st.Status)
	}

	now := time.Now().UTC()
	if err := s.repo.UpdateStatus(ctx, settlementID, domain.SettlementStatusSettled, &now); err != nil {
		return nil, err
	}
	st.Status = domain.SettlementStatusSettled
	st.SettledAt = &now

	if s.ledgerSvc != nil {
		if err := s.ledgerSvc.Debit(ctx, st.OwnerUserID, st.NetAmount, domain.LedgerEntryTypeSettlementPayout, "settlement", settlementID); err != nil {
			return nil, err
		}
	}
	return st, nil
}

// List 返回号主结算单列表。
func (s *SettlementService) List(ctx context.Context, ownerID int64, params pagination.PaginationParams) ([]Settlement, *pagination.PaginationResult, error) {
	if s == nil || s.repo == nil {
		return nil, nil, fmt.Errorf("settlement service repo is nil")
	}
	return s.repo.ListByOwner(ctx, ownerID, params)
}

// ListAll 返回全部结算单（管理面）。
func (s *SettlementService) ListAll(ctx context.Context, params pagination.PaginationParams) ([]Settlement, *pagination.PaginationResult, error) {
	if s == nil || s.repo == nil {
		return nil, nil, fmt.Errorf("settlement service repo is nil")
	}
	return s.repo.ListAll(ctx, params)
}

// ExportCSV 将结算单导出为 CSV 字节流（标准库 encoding/csv）。
func (s *SettlementService) ExportCSV(ctx context.Context, start, end time.Time) ([]byte, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("settlement service repo is nil")
	}
	rows, err := s.repo.ListByPeriod(ctx, start, end)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write([]string{"settlement_no", "owner_user_id", "period_start", "period_end", "gross_amount", "fee_amount", "net_amount", "status", "settled_at"}); err != nil {
		return nil, err
	}
	for _, r := range rows {
		settledAt := ""
		if r.SettledAt != nil {
			settledAt = r.SettledAt.Format(time.RFC3339)
		}
		record := []string{
			r.SettlementNo,
			fmt.Sprintf("%d", r.OwnerUserID),
			r.PeriodStart.Format(time.RFC3339),
			r.PeriodEnd.Format(time.RFC3339),
			fmt.Sprintf("%.8f", r.GrossAmount),
			fmt.Sprintf("%.8f", r.FeeAmount),
			fmt.Sprintf("%.8f", r.NetAmount),
			r.Status,
			settledAt,
		}
		if err := w.Write(record); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
