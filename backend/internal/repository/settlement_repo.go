// Package repository 实现数据访问层（Repository Pattern）。
package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/your-org/ai-gateway-platform/internal/pkg/pagination"
	"github.com/your-org/ai-gateway-platform/internal/service"
)

// settlementRepository 实现 service.SettlementRepository 接口（原生 SQL）。
type settlementRepository struct {
	db *sql.DB
}

// NewSettlementRepository 构造结算单仓储。
func NewSettlementRepository(sqlDB *sql.DB) service.SettlementRepository {
	return &settlementRepository{db: sqlDB}
}

func (r *settlementRepository) Create(ctx context.Context, s *service.Settlement) error {
	if r == nil || r.db == nil {
		return errors.New("settlement repository db is nil")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO settlements (owner_user_id, settlement_no, period_start, period_end, gross_amount, fee_amount, net_amount, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		ON CONFLICT (settlement_no) DO NOTHING
	`, s.OwnerUserID, s.SettlementNo, s.PeriodStart, s.PeriodEnd, s.GrossAmount, s.FeeAmount, s.NetAmount, s.Status)
	return err
}

func (r *settlementRepository) GetByID(ctx context.Context, id int64) (*service.Settlement, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("settlement repository db is nil")
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, owner_user_id, settlement_no, period_start, period_end, gross_amount, fee_amount, net_amount, status, settled_at, created_at, updated_at
		FROM settlements WHERE id = $1
	`, id)
	return scanSettlement(row)
}

func (r *settlementRepository) ListByOwner(ctx context.Context, ownerID int64, params pagination.PaginationParams) ([]service.Settlement, *pagination.PaginationResult, error) {
	if r == nil || r.db == nil {
		return nil, nil, errors.New("settlement repository db is nil")
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM settlements WHERE owner_user_id = $1`, ownerID).Scan(&total); err != nil {
		return nil, nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, owner_user_id, settlement_no, period_start, period_end, gross_amount, fee_amount, net_amount, status, settled_at, created_at, updated_at
		FROM settlements WHERE owner_user_id = $1 ORDER BY id DESC LIMIT $2 OFFSET $3
	`, ownerID, params.Limit(), params.Offset())
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := make([]service.Settlement, 0)
	for rows.Next() {
		s, err := scanSettlement(rows)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, *s)
	}
	return items, &pagination.PaginationResult{Total: total, Page: params.Page, PageSize: params.Limit()}, rows.Err()
}

func (r *settlementRepository) UpdateStatus(ctx context.Context, id int64, status string, settledAt *time.Time) error {
	if r == nil || r.db == nil {
		return errors.New("settlement repository db is nil")
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE settlements SET status = $1, settled_at = $2, updated_at = NOW() WHERE id = $3
	`, status, settledAt, id)
	return err
}

func (r *settlementRepository) ListByPeriod(ctx context.Context, start, end time.Time) ([]service.Settlement, error) {	if r == nil || r.db == nil {
		return nil, errors.New("settlement repository db is nil")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, owner_user_id, settlement_no, period_start, period_end, gross_amount, fee_amount, net_amount, status, settled_at, created_at, updated_at
		FROM settlements WHERE period_start >= $1 AND period_end <= $2 ORDER BY id ASC
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]service.Settlement, 0)
	for rows.Next() {
		s, err := scanSettlement(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *s)
	}
	return items, rows.Err()
}

func (r *settlementRepository) ListAll(ctx context.Context, params pagination.PaginationParams) ([]service.Settlement, *pagination.PaginationResult, error) {
	if r == nil || r.db == nil {
		return nil, nil, errors.New("settlement repository db is nil")
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM settlements`).Scan(&total); err != nil {
		return nil, nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, owner_user_id, settlement_no, period_start, period_end, gross_amount, fee_amount, net_amount, status, settled_at, created_at, updated_at
		FROM settlements ORDER BY id DESC LIMIT $1 OFFSET $2
	`, params.Limit(), params.Offset())
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := make([]service.Settlement, 0)
	for rows.Next() {
		s, err := scanSettlement(rows)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, *s)
	}
	return items, &pagination.PaginationResult{Total: total, Page: params.Page, PageSize: params.Limit()}, rows.Err()
}

func (r *settlementRepository) SumShareCreditByOwner(ctx context.Context, start, end time.Time) ([]service.OwnerShareCreditAgg, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("settlement repository db is nil")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT owner_user_id, COALESCE(SUM(amount), 0)
		FROM owner_ledgers
		WHERE entry_type = 'share_credit' AND created_at >= $1 AND created_at < $2
		GROUP BY owner_user_id
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]service.OwnerShareCreditAgg, 0)
	for rows.Next() {
		var agg service.OwnerShareCreditAgg
		if err := rows.Scan(&agg.OwnerUserID, &agg.Total); err != nil {
			return nil, err
		}
		out = append(out, agg)
	}
	return out, rows.Err()
}

type settlementScanner interface {
	Scan(dest ...any) error
}

func scanSettlement(sc settlementScanner) (*service.Settlement, error) {
	var s service.Settlement
	var settledAt sql.NullTime
	if err := sc.Scan(
		&s.ID, &s.OwnerUserID, &s.SettlementNo, &s.PeriodStart, &s.PeriodEnd,
		&s.GrossAmount, &s.FeeAmount, &s.NetAmount, &s.Status, &settledAt, &s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if settledAt.Valid {
		s.SettledAt = &settledAt.Time
	}
	return &s, nil
}
