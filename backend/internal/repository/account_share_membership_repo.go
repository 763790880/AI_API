// Package repository 实现数据访问层（Repository Pattern）。
package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/your-org/ai-gateway-platform/internal/service"
)

// accountShareMembershipRepository 实现 service.AccountShareMembershipRepository 接口（原生 SQL）。
type accountShareMembershipRepository struct {
	db *sql.DB
}

// NewAccountShareMembershipRepository 构造广场会员仓储。
func NewAccountShareMembershipRepository(sqlDB *sql.DB) service.AccountShareMembershipRepository {
	return &accountShareMembershipRepository{db: sqlDB}
}

func (r *accountShareMembershipRepository) Create(ctx context.Context, m *service.AccountShareMembership) error {
	if r == nil || r.db == nil {
		return errors.New("account share membership repository db is nil")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO account_share_memberships (room_id, account_id, consumer_user_id, api_key_id, status, joined_at, ended_at, queue_expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
	`, m.RoomID, m.AccountID, m.ConsumerUserID, m.APIKeyID, m.Status, m.JoinedAt, m.EndedAt, m.QueueExpiresAt)
	return err
}

func (r *accountShareMembershipRepository) GetByID(ctx context.Context, id int64) (*service.AccountShareMembership, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("account share membership repository db is nil")
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, room_id, account_id, consumer_user_id, api_key_id, status, joined_at, ended_at, queue_expires_at, created_at, updated_at
		FROM account_share_memberships WHERE id = $1
	`, id)
	return scanMembership(row)
}

func (r *accountShareMembershipRepository) UpdateStatus(ctx context.Context, id int64, status string, endedAt, queueExpiresAt *time.Time) error {
	if r == nil || r.db == nil {
		return errors.New("account share membership repository db is nil")
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE account_share_memberships SET status = $1, ended_at = $2, queue_expires_at = $3, updated_at = NOW() WHERE id = $4
	`, status, endedAt, queueExpiresAt, id)
	return err
}

func (r *accountShareMembershipRepository) CountActiveSeats(ctx context.Context, roomID int64) (int, error) {
	return r.count(ctx, roomID, `status = 'active'`)
}

func (r *accountShareMembershipRepository) CountQueued(ctx context.Context, roomID int64) (int, error) {
	return r.count(ctx, roomID, `status = 'queued'`)
}

func (r *accountShareMembershipRepository) count(ctx context.Context, roomID int64, cond string) (int, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("account share membership repository db is nil")
	}
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_share_memberships WHERE room_id = $1 AND `+cond, roomID).Scan(&n)
	return n, err
}

func (r *accountShareMembershipRepository) ListByRoom(ctx context.Context, roomID int64) ([]service.AccountShareMembership, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("account share membership repository db is nil")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, room_id, account_id, consumer_user_id, api_key_id, status, joined_at, ended_at, queue_expires_at, created_at, updated_at
		FROM account_share_memberships WHERE room_id = $1 ORDER BY id ASC
	`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]service.AccountShareMembership, 0)
	for rows.Next() {
		m, err := scanMembership(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func scanMembership(sc settlementScanner) (*service.AccountShareMembership, error) {
	var m service.AccountShareMembership
	var apiKeyID sql.NullInt64
	var endedAt, queueExpiresAt sql.NullTime
	if err := sc.Scan(
		&m.ID, &m.RoomID, &m.AccountID, &m.ConsumerUserID, &apiKeyID, &m.Status, &m.JoinedAt, &endedAt, &queueExpiresAt, &m.CreatedAt, &m.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if apiKeyID.Valid {
		m.APIKeyID = &apiKeyID.Int64
	}
	if endedAt.Valid {
		m.EndedAt = &endedAt.Time
	}
	if queueExpiresAt.Valid {
		m.QueueExpiresAt = &queueExpiresAt.Time
	}
	return &m, nil
}
