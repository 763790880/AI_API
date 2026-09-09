// Package service provides business logic and domain services for the application.
package service

import (
	"context"
	"time"

	"github.com/your-org/ai-gateway-platform/internal/domain"
)

// DefaultOwnerShareRatio 是未配置全局策略时的默认号主分成比例。
const DefaultOwnerShareRatio = 0.7

// SharePolicy 分账策略领域对象。
type SharePolicy struct {
	ID                int64
	ScopeType         string
	ScopeID           *int64
	Platform          *string
	OwnerShareRatio   float64
	InviteShareRatio  float64
	Version           int
	Enabled           bool
	EffectiveAt       *time.Time
	CreatedByAdminID  *int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// SharePolicyRepository 定义分账策略的数据访问接口。
type SharePolicyRepository interface {
	GetEnabledGlobal(ctx context.Context) (float64, error)
	List(ctx context.Context) ([]SharePolicy, error)
	GetByID(ctx context.Context, id int64) (*SharePolicy, error)
	Create(ctx context.Context, p *SharePolicy) error
	Update(ctx context.Context, p *SharePolicy) error
}

// SharePolicyService 提供分账策略解析与管理能力。
type SharePolicyService struct {
	repo SharePolicyRepository
}

// NewSharePolicyService 构造分账策略服务。
func NewSharePolicyService(repo SharePolicyRepository) *SharePolicyService {
	return &SharePolicyService{repo: repo}
}

// ResolveEnabledRatio 返回当前生效的号主分成比例。
// 未配置全局策略或比例非法时回退到 DefaultOwnerShareRatio。
func (s *SharePolicyService) ResolveEnabledRatio(ctx context.Context) (float64, error) {
	if s == nil || s.repo == nil {
		return DefaultOwnerShareRatio, nil
	}
	ratio, err := s.repo.GetEnabledGlobal(ctx)
	if err != nil {
		return 0, err
	}
	if ratio <= 0 || ratio > 1 {
		return DefaultOwnerShareRatio, nil
	}
	return ratio, nil
}

// List 返回全部分账策略。
func (s *SharePolicyService) List(ctx context.Context) ([]SharePolicy, error) {
	if s == nil || s.repo == nil {
		return []SharePolicy{}, nil
	}
	return s.repo.List(ctx)
}

// GetByID 按 ID 返回分账策略。
func (s *SharePolicyService) GetByID(ctx context.Context, id int64) (*SharePolicy, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	return s.repo.GetByID(ctx, id)
}

// Create 创建分账策略。
func (s *SharePolicyService) Create(ctx context.Context, p *SharePolicy) (*SharePolicy, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	if p == nil {
		return nil, nil
	}
	if p.ScopeType == "" {
		p.ScopeType = domain.SharePolicyScopeGlobal
	}
	if p.OwnerShareRatio <= 0 || p.OwnerShareRatio > 1 {
		p.OwnerShareRatio = DefaultOwnerShareRatio
	}
	if p.Version <= 0 {
		p.Version = 1
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Update 更新分账策略。
func (s *SharePolicyService) Update(ctx context.Context, id int64, p *SharePolicy) (*SharePolicy, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}
	if p.ScopeType != "" {
		existing.ScopeType = p.ScopeType
	}
	if p.ScopeID != nil {
		existing.ScopeID = p.ScopeID
	}
	if p.Platform != nil {
		existing.Platform = p.Platform
	}
	if p.OwnerShareRatio > 0 && p.OwnerShareRatio <= 1 {
		existing.OwnerShareRatio = p.OwnerShareRatio
	}
	if p.Version > 0 {
		existing.Version = p.Version
	}
	existing.Enabled = p.Enabled
	if p.EffectiveAt != nil {
		existing.EffectiveAt = p.EffectiveAt
	}
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}
