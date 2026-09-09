// Package schema 定义 Ent ORM 的数据库 schema。
package schema

import (
	"github.com/your-org/ai-gateway-platform/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SharePolicy 定义分账策略实体 schema。
//
// 分账策略决定号主与平台之间的收益分成比例。支持 global/platform/account/group
// 四级作用域，由 SharePolicyService.ResolveEnabled 按优先级解析生效策略。
type SharePolicy struct {
	ent.Schema
}

// Annotations 指定数据库表名为 "share_policies"。
func (SharePolicy) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "share_policies"},
	}
}

// Mixin 使用时间戳与软删除混入组件。
func (SharePolicy) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

// Fields 定义分账策略实体的所有字段。
func (SharePolicy) Fields() []ent.Field {
	return []ent.Field{
		// scope_type: 策略作用域类型。global/platform/account/group。
		field.String("scope_type").
			MaxLen(20).
			NotEmpty().
			Comment("Policy scope type: global/platform/account/group."),
		// scope_id: 作用域实体 ID（platform/account/group 时有效；global 时为 NULL）。
		field.Int64("scope_id").
			Optional().
			Nillable().
			Comment("Scope entity id (NULL for global scope)."),
		// platform: 平台维度覆盖（NULL 表示不区分平台）。
		field.String("platform").
			MaxLen(50).
			Optional().
			Nillable().
			Comment("Platform override (NULL = all platforms)."),
		// owner_share_ratio: 号主分成比例，取值 [0,1]。
		field.Float("owner_share_ratio").
			SchemaType(map[string]string{dialect.Postgres: "decimal(10,4)"}).
			Range(0, 1).
			Comment("Owner revenue share ratio in [0,1]."),
		// invite_share_ratio: 邀请人分成比例（预留，P2 实现）。
		field.Float("invite_share_ratio").
			SchemaType(map[string]string{dialect.Postgres: "decimal(10,4)"}).
			Default(0).
			Comment("Inviter revenue share ratio (reserved)."),
		// version: 策略版本号，用于乐观锁。
		field.Int("version").
			Default(1).
			Comment("Policy version for optimistic locking."),
		// enabled: 是否启用该策略。
		field.Bool("enabled").
			Default(true),
		// effective_at: 策略生效时间（NULL 表示立即生效）。
		field.Time("effective_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		// created_by_admin_id: 创建该策略的管理员用户 ID。
		field.Int64("created_by_admin_id").
			Optional().
			Nillable(),
	}
}

// Indexes 定义分账策略的查询索引。
func (SharePolicy) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("scope_type", "scope_id"),
		index.Fields("platform"),
		index.Fields("enabled"),
		index.Fields("deleted_at"),
	}
}
