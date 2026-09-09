// Package schema 定义 Ent ORM 的数据库 schema。
package schema

import (
	"time"

	"github.com/your-org/ai-gateway-platform/ent/schema/mixins"
	"github.com/your-org/ai-gateway-platform/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AccountShareMembership 定义账号广场会员（座位/排队）实体 schema。
//
// 记录用户加入广场房间后的会员关系，覆盖使用中/排队/结束等生命周期状态。
type AccountShareMembership struct {
	ent.Schema
}

// Annotations 指定数据库表名为 "account_share_memberships"。
func (AccountShareMembership) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "account_share_memberships"},
	}
}

// Mixin 使用时间戳混入组件（生命周期由 status 字段追踪）。
func (AccountShareMembership) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

// Fields 定义广场会员实体的所有字段。
func (AccountShareMembership) Fields() []ent.Field {
	return []ent.Field{
		// room_id: 关联的广场房间 ID。
		field.Int64("room_id"),
		// account_id: 冗余关联的托管账号 ID（便于按账号统计）。
		field.Int64("account_id"),
		// consumer_user_id: 消费用户（座位使用者）ID。
		field.Int64("consumer_user_id"),
		// api_key_id: 消费用户使用的 API Key ID（可选）。
		field.Int64("api_key_id").
			Optional().
			Nillable(),
		// status: 会员状态。active/queued/ended/expired。
		field.String("status").
			MaxLen(20).
			Default(domain.MembershipStatusActive),
		// joined_at: 加入时间。
		field.Time("joined_at").
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		// ended_at: 结束时间（退座/被踢/过期）。
		field.Time("ended_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		// queue_expires_at: 排队过期时间。
		field.Time("queue_expires_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

// Indexes 定义广场会员的查询索引。
func (AccountShareMembership) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("room_id"),
		index.Fields("account_id"),
		index.Fields("consumer_user_id"),
		index.Fields("api_key_id"),
		index.Fields("status"),
		index.Fields("queue_expires_at"),
	}
}
