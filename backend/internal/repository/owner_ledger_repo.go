// Package repository 实现数据访问层（Repository Pattern）。
package repository

import (
	"context"
	"database/sql"
	"errors"

	dbent "github.com/your-org/ai-gateway-platform/ent"
	dbownerledger "github.com/your-org/ai-gateway-platform/ent/ownerledger"
	"github.com/your-org/ai-gateway-platform/internal/pkg/pagination"
	"github.com/your-org/ai-gateway-platform/internal/service"
)

// ownerLedgerRepository 实现 service.OwnerLedgerRepository 接口。
type ownerLedgerRepository struct {
	client *dbent.Client
	sql    *sql.DB
}

// NewOwnerLedgerRepository 构造号主收益账本仓储。
func NewOwnerLedgerRepository(client *dbent.Client, sqlDB *sql.DB) service.OwnerLedgerRepository {
	return &ownerLedgerRepository{client: client, sql: sqlDB}
}

// Credit 写入一条账本流水（direction 由 entry.Direction 决定）。
// 重复的 (ref_type, ref_id) 由部分唯一索引拦截并静默跳过（幂等）。
func (r *ownerLedgerRepository) Credit(ctx context.Context, entry service.OwnerLedgerEntry) error {
	if r == nil || r.client == nil {
		return errors.New("owner ledger repository client is nil")
	}

	create := r.client.OwnerLedger.Create().
		SetOwnerUserID(entry.OwnerUserID).
		SetDirection(dbownerledger.Direction(entry.Direction)).
		SetAmount(entry.Amount).
		SetBalanceBefore(entry.BalanceBefore).
		SetBalanceAfter(entry.BalanceAfter).
		SetEntryType(entry.EntryType)
	if entry.RefType != "" {
		create.SetRefType(entry.RefType)
	}
	if entry.RefID != 0 {
		create.SetRefID(entry.RefID)
	}
	if entry.Metadata != nil {
		create.SetMetadata(entry.Metadata)
	}

	if _, err := create.Save(ctx); err != nil {
		if dbent.IsConstraintError(err) {
			// (ref_type, ref_id) 部分唯一索引冲突 → 同一业务对象已入账，幂等跳过。
			return nil
		}
		return err
	}
	return nil
}

// Balance 返回号主收益余额双值：available（可提现）、pending（待结算）。
//
// 语义：
//   - pending = share_credit - settlement_payout
//   - available = settlement_payout - withdrawal_hold + withdrawal_release
//
// 按 entry_type 分组聚合（entry_type 已蕴含方向语义），不走简单的 credit-debit 差值。
func (r *ownerLedgerRepository) Balance(ctx context.Context, ownerID int64) (available, pending float64, err error) {
	if r == nil || r.sql == nil {
		return 0, 0, errors.New("owner ledger repository sql is nil")
	}

	var shareCredit, settlementPayout, withdrawalHold, withdrawalRelease float64
	err = r.sql.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN entry_type = 'share_credit'       THEN amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN entry_type = 'settlement_payout'  THEN amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN entry_type = 'withdrawal_hold'    THEN amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN entry_type = 'withdrawal_release' THEN amount ELSE 0 END), 0)
		FROM owner_ledgers
		WHERE owner_user_id = $1
	`, ownerID).Scan(&shareCredit, &settlementPayout, &withdrawalHold, &withdrawalRelease)
	if err != nil {
		return 0, 0, err
	}

	pending = shareCredit - settlementPayout
	available = settlementPayout - withdrawalHold + withdrawalRelease
	if pending < 0 {
		pending = 0
	}
	if available < 0 {
		available = 0
	}
	return available, pending, nil
}

// ListByOwner 返回号主收益流水。
func (r *ownerLedgerRepository) ListByOwner(ctx context.Context, ownerID int64, params pagination.PaginationParams) ([]service.OwnerLedger, *pagination.PaginationResult, error) {
	if r == nil || r.sql == nil {
		return nil, nil, errors.New("owner ledger repository sql is nil")
	}
	var total int64
	if err := r.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM owner_ledgers WHERE owner_user_id = $1`, ownerID).Scan(&total); err != nil {
		return nil, nil, err
	}
	rows, err := r.sql.QueryContext(ctx, `
		SELECT id, owner_user_id, direction, amount, balance_before, balance_after, entry_type, ref_type, ref_id, created_at
		FROM owner_ledgers WHERE owner_user_id = $1 ORDER BY id DESC LIMIT $2 OFFSET $3
	`, ownerID, params.Limit(), params.Offset())
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	out := make([]service.OwnerLedger, 0)
	for rows.Next() {
		var l service.OwnerLedger
		var refType sql.NullString
		var refID sql.NullInt64
		if err := rows.Scan(&l.ID, &l.OwnerUserID, &l.Direction, &l.Amount, &l.BalanceBefore, &l.BalanceAfter, &l.EntryType, &refType, &refID, &l.CreatedAt); err != nil {
			return nil, nil, err
		}
		if refType.Valid {
			l.RefType = &refType.String
		}
		if refID.Valid {
			l.RefID = &refID.Int64
		}
		out = append(out, l)
	}
	return out, &pagination.PaginationResult{Total: total, Page: params.Page, PageSize: params.Limit()}, rows.Err()
}
