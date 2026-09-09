// Package schema 定义 Ent ORM 的数据库 schema。
package schema

import (
	"github.com/your-org/ai-gateway-platform/ent/schema/mixins"
	"github.com/your-org/ai-gateway-platform/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Settlement 定义结算单实体 schema。
//
// 结算单按月/周期汇总号主收益，记录毛额、手续费与净额，是提现的依据。
type Settlement struct {
	ent.Schema
}

// Annotations 指定数据库表名为 "settlements"。
func (Settlement) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "settlements"},
	}
}

// Mixin 使用时间戳混入组件（生命周期由 status 字段追踪，不做软删除）。
func (Settlement) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

// Fields 定义结算单实体的所有字段。
func (Settlement) Fields() []ent.Field {
	return []ent.Field{
		// owner_user_id: 结算归属号主用户 ID。
		field.Int64("owner_user_id"),
		// settlement_no: 结算单号，全局唯一。
		field.String("settlement_no").
			MaxLen(64).
			NotEmpty().
			Unique(),
		// period_start / period_end: 结算周期（UTC）。
		field.Time("period_start").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("period_end").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		// gross_amount: 周期内分成毛额。
		field.Float("gross_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).
			Default(0),
		// fee_amount: 平台手续费。
		field.Float("fee_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).
			Default(0),
		// net_amount: 结算净额（gross - fee）。
		field.Float("net_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).
			Default(0),
		// status: 结算状态。pending/settled/failed。
		field.String("status").
			MaxLen(20).
			Default(domain.SettlementStatusPending),
		// settled_at: 实际结算时间。
		field.Time("settled_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

// Indexes 定义结算单的查询索引。
func (Settlement) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_user_id"),
		index.Fields("owner_user_id", "status"),
		index.Fields("status"),
		index.Fields("period_start", "period_end"),
	}
}
