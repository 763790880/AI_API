// Package repository 实现数据访问层（Repository Pattern）。
package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"
	"github.com/your-org/ai-gateway-platform/internal/pkg/pagination"
	"github.com/your-org/ai-gateway-platform/internal/service"
)

// withdrawalRepository 实现 service.WithdrawalRepository 接口（原生 SQL）。
type withdrawalRepository struct {
	db *sql.DB
}

// NewWithdrawalRepository 构造提现单仓储。
func NewWithdrawalRepository(sqlDB *sql.DB) service.WithdrawalRepository {
	return &withdrawalRepository{db: sqlDB}
}

func (r *withdrawalRepository) Create(ctx context.Context, w *service.Withdrawal) error {
	if r == nil || r.db == nil {
		return errors.New("withdrawal repository db is nil")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO withdrawals (owner_user_id, amount, fee_amount, total_deducted, balance_before, balance_after, payment_method, payment_account, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
	`, w.OwnerUserID, w.Amount, w.FeeAmount, w.TotalDeducted, w.BalanceBefore, w.BalanceAfter, w.PaymentMethod, w.PaymentAccount, w.Status)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			// withdrawals_owner_pending_uq 部分唯一索引 → 每号主最多一笔 pending。
			return service.ErrWithdrawalPendingExists
		}
		return err
	}
	return nil
}

func (r *withdrawalRepository) GetByID(ctx context.Context, id int64) (*service.Withdrawal, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("withdrawal repository db is nil")
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, owner_user_id, amount, fee_amount, total_deducted, balance_before, balance_after, payment_method, payment_account, status, processed_by_user_id, processed_at, created_at, updated_at
		FROM withdrawals WHERE id = $1
	`, id)
	return scanWithdrawal(row)
}

func (r *withdrawalRepository) ListByOwner(ctx context.Context, ownerID int64, params pagination.PaginationParams) ([]service.Withdrawal, *pagination.PaginationResult, error) {
	return r.list(ctx, `WHERE owner_user_id = $1`, ownerID, params)
}

func (r *withdrawalRepository) ListAll(ctx context.Context, params pagination.PaginationParams) ([]service.Withdrawal, *pagination.PaginationResult, error) {
	return r.list(ctx, ``, nil, params)
}

func (r *withdrawalRepository) list(ctx context.Context, where string, ownerID any, params pagination.PaginationParams) ([]service.Withdrawal, *pagination.PaginationResult, error) {
	if r == nil || r.db == nil {
		return nil, nil, errors.New("withdrawal repository db is nil")
	}

	var total int64
	var countArgs []any
	if where != "" {
		countArgs = append(countArgs, ownerID)
	}
	var err error
	if where != "" {
		err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM withdrawals `+where, countArgs...).Scan(&total)
	} else {
		err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM withdrawals`).Scan(&total)
	}
	if err != nil {
		return nil, nil, err
	}

	query := `SELECT id, owner_user_id, amount, fee_amount, total_deducted, balance_before, balance_after, payment_method, payment_account, status, processed_by_user_id, processed_at, created_at, updated_at FROM withdrawals ` + where + ` ORDER BY id DESC LIMIT $2 OFFSET $3`
	args := []any{ownerID, params.Limit(), params.Offset()}
	if where == "" {
		query = `SELECT id, owner_user_id, amount, fee_amount, total_deducted, balance_before, balance_after, payment_method, payment_account, status, processed_by_user_id, processed_at, created_at, updated_at FROM withdrawals ORDER BY id DESC LIMIT $1 OFFSET $2`
		args = []any{params.Limit(), params.Offset()}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := make([]service.Withdrawal, 0)
	for rows.Next() {
		w, err := scanWithdrawal(rows)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, *w)
	}
	return items, &pagination.PaginationResult{Total: total, Page: params.Page, PageSize: params.Limit()}, rows.Err()
}

func (r *withdrawalRepository) UpdateStatus(ctx context.Context, id int64, status string, processedBy *int64, processedAt *time.Time) error {
	if r == nil || r.db == nil {
		return errors.New("withdrawal repository db is nil")
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE withdrawals SET status = $1, processed_by_user_id = $2, processed_at = $3, updated_at = NOW() WHERE id = $4
	`, status, processedBy, processedAt, id)
	return err
}

func scanWithdrawal(sc settlementScanner) (*service.Withdrawal, error) {
	var w service.Withdrawal
	var processedBy sql.NullInt64
	var processedAt sql.NullTime
	if err := sc.Scan(
		&w.ID, &w.OwnerUserID, &w.Amount, &w.FeeAmount, &w.TotalDeducted, &w.BalanceBefore, &w.BalanceAfter,
		&w.PaymentMethod, &w.PaymentAccount, &w.Status, &processedBy, &processedAt, &w.CreatedAt, &w.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if processedBy.Valid {
		w.ProcessedByUserID = &processedBy.Int64
	}
	if processedAt.Valid {
		w.ProcessedAt = &processedAt.Time
	}
	return &w, nil
}
