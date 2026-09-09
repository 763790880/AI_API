// Package repository 实现数据访问层（Repository Pattern）。
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/lib/pq"
	"github.com/your-org/ai-gateway-platform/internal/service"
)

// accountShareRoomRepository 实现 service.AccountShareRoomRepository 接口（原生 SQL）。
type accountShareRoomRepository struct {
	db *sql.DB
}

// NewAccountShareRoomRepository 构造广场房间仓储。
func NewAccountShareRoomRepository(sqlDB *sql.DB) service.AccountShareRoomRepository {
	return &accountShareRoomRepository{db: sqlDB}
}

func (r *accountShareRoomRepository) Create(ctx context.Context, room *service.AccountShareRoom) error {
	if r == nil || r.db == nil {
		return errors.New("account share room repository db is nil")
	}
	modelsJSON, _ := json.Marshal(room.AllowedModels)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO account_share_rooms (account_id, owner_user_id, name, status, seat_limit, rate_multiplier, allowed_models, per_user_concurrency, queue_max, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 0, NOW(), NOW())
	`, room.AccountID, room.OwnerUserID, room.Name, room.Status, room.SeatLimit, room.RateMultiplier, modelsJSON, room.PerUserConcurrency, room.QueueMax)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return service.ErrRoomAccountConflict
		}
		return err
	}
	return nil
}

func (r *accountShareRoomRepository) GetByID(ctx context.Context, id int64) (*service.AccountShareRoom, error) {
	return r.get(ctx, `WHERE id = $1`, id)
}

func (r *accountShareRoomRepository) GetByAccountID(ctx context.Context, accountID int64) (*service.AccountShareRoom, error) {
	return r.get(ctx, `WHERE account_id = $1`, accountID)
}

func (r *accountShareRoomRepository) get(ctx context.Context, where string, arg any) (*service.AccountShareRoom, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("account share room repository db is nil")
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, account_id, owner_user_id, name, status, seat_limit, rate_multiplier, allowed_models, per_user_concurrency, queue_max, version, created_at, updated_at
		FROM account_share_rooms `+where, arg)
	return scanRoom(row)
}

func (r *accountShareRoomRepository) Update(ctx context.Context, room *service.AccountShareRoom) error {
	if r == nil || r.db == nil {
		return errors.New("account share room repository db is nil")
	}
	modelsJSON, _ := json.Marshal(room.AllowedModels)
	res, err := r.db.ExecContext(ctx, `
		UPDATE account_share_rooms
		SET name = $1, status = $2, seat_limit = $3, rate_multiplier = $4, allowed_models = $5, per_user_concurrency = $6, queue_max = $7, version = version + 1, updated_at = NOW()
		WHERE id = $8 AND version = $9
	`, room.Name, room.Status, room.SeatLimit, room.RateMultiplier, modelsJSON, room.PerUserConcurrency, room.QueueMax, room.ID, room.Version)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return service.ErrRoomVersionConflict
	}
	room.Version++
	return nil
}

func (r *accountShareRoomRepository) ListActive(ctx context.Context) ([]service.AccountShareRoom, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("account share room repository db is nil")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, account_id, owner_user_id, name, status, seat_limit, rate_multiplier, allowed_models, per_user_concurrency, queue_max, version, created_at, updated_at
		FROM account_share_rooms WHERE status = 'active' ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]service.AccountShareRoom, 0)
	for rows.Next() {
		room, err := scanRoom(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *room)
	}
	return out, rows.Err()
}

func scanRoom(sc settlementScanner) (*service.AccountShareRoom, error) {
	var r service.AccountShareRoom
	var modelsJSON []byte
	if err := sc.Scan(
		&r.ID, &r.AccountID, &r.OwnerUserID, &r.Name, &r.Status, &r.SeatLimit, &r.RateMultiplier, &modelsJSON,
		&r.PerUserConcurrency, &r.QueueMax, &r.Version, &r.CreatedAt, &r.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if len(modelsJSON) > 0 {
		_ = json.Unmarshal(modelsJSON, &r.AllowedModels)
	}
	if r.AllowedModels == nil {
		r.AllowedModels = []string{}
	}
	return &r, nil
}
