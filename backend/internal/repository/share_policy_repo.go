// Package repository 实现数据访问层（Repository Pattern）。
package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/your-org/ai-gateway-platform/internal/service"
)

// sharePolicyRepository 实现 service.SharePolicyRepository 接口（原生 SQL）。
type sharePolicyRepository struct {
	db *sql.DB
}

// NewSharePolicyRepository 构造分账策略仓储。
func NewSharePolicyRepository(sqlDB *sql.DB) service.SharePolicyRepository {
	return &sharePolicyRepository{db: sqlDB}
}

// GetEnabledGlobal 返回启用中的全局分账策略的 owner_share_ratio（最新一条）。
func (r *sharePolicyRepository) GetEnabledGlobal(ctx context.Context) (float64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("share policy repository db is nil")
	}
	var ratio float64
	err := r.db.QueryRowContext(ctx, `
		SELECT owner_share_ratio FROM share_policies
		WHERE scope_type = 'global' AND enabled = TRUE AND deleted_at IS NULL
		ORDER BY id DESC LIMIT 1
	`).Scan(&ratio)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return ratio, err
}

func (r *sharePolicyRepository) List(ctx context.Context) ([]service.SharePolicy, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("share policy repository db is nil")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, scope_type, scope_id, platform, owner_share_ratio, invite_share_ratio, version, enabled, effective_at, created_by_admin_id, created_at, updated_at
		FROM share_policies WHERE deleted_at IS NULL ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]service.SharePolicy, 0)
	for rows.Next() {
		p, err := scanSharePolicy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (r *sharePolicyRepository) GetByID(ctx context.Context, id int64) (*service.SharePolicy, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("share policy repository db is nil")
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, scope_type, scope_id, platform, owner_share_ratio, invite_share_ratio, version, enabled, effective_at, created_by_admin_id, created_at, updated_at
		FROM share_policies WHERE id = $1 AND deleted_at IS NULL
	`, id)
	p, err := scanSharePolicy(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

func (r *sharePolicyRepository) Create(ctx context.Context, p *service.SharePolicy) error {
	if r == nil || r.db == nil {
		return errors.New("share policy repository db is nil")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO share_policies (scope_type, scope_id, platform, owner_share_ratio, invite_share_ratio, version, enabled, effective_at, created_by_admin_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
	`, p.ScopeType, p.ScopeID, p.Platform, p.OwnerShareRatio, p.InviteShareRatio, p.Version, p.Enabled, p.EffectiveAt, p.CreatedByAdminID)
	return err
}

func (r *sharePolicyRepository) Update(ctx context.Context, p *service.SharePolicy) error {
	if r == nil || r.db == nil {
		return errors.New("share policy repository db is nil")
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE share_policies
		SET scope_type = $1, scope_id = $2, platform = $3, owner_share_ratio = $4, invite_share_ratio = $5, version = $6, enabled = $7, effective_at = $8, updated_at = NOW()
		WHERE id = $9 AND deleted_at IS NULL
	`, p.ScopeType, p.ScopeID, p.Platform, p.OwnerShareRatio, p.InviteShareRatio, p.Version, p.Enabled, p.EffectiveAt, p.ID)
	return err
}

func scanSharePolicy(sc settlementScanner) (*service.SharePolicy, error) {
	var p service.SharePolicy
	var scopeID sql.NullInt64
	var platform sql.NullString
	var effectiveAt sql.NullTime
	var adminID sql.NullInt64
	if err := sc.Scan(
		&p.ID, &p.ScopeType, &scopeID, &platform, &p.OwnerShareRatio, &p.InviteShareRatio, &p.Version, &p.Enabled, &effectiveAt, &adminID, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if scopeID.Valid {
		p.ScopeID = &scopeID.Int64
	}
	if platform.Valid {
		p.Platform = &platform.String
	}
	if effectiveAt.Valid {
		p.EffectiveAt = &effectiveAt.Time
	}
	if adminID.Valid {
		p.CreatedByAdminID = &adminID.Int64
	}
	return &p, nil
}
