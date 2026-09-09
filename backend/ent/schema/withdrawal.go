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

// Withdrawal 定义提现单实体 schema。
//
// 提现单记录号主发起提现、平台审核打款的完整流程。
type Withdrawal struct {
	ent.Schema
}

// Annotations 指定数据库表名为 "withdrawals"。
func (Withdrawal) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "withdrawals"},
	}
}

// Mixin 使用时间戳混入组件（生命周期由 status 字段追踪，不做软删除）。
func (Withdrawal) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

// Fields 定义提现单实体的所有字段。
func (Withdrawal) Fields() []ent.Field {
	return []ent.Field{
		// owner_user_id: 发起提现的号主用户 ID。
		field.Int64("owner_user_id"),
		// amount: 提现金额。
		field.Float("amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		// fee_amount: 提现手续费。
		field.Float("fee_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).
			Default(0),
		// total_deducted: 实际扣减总额（amount + fee_amount）。
		field.Float("total_deducted").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).
			Default(0),
		// balance_before / balance_after: 扣减前后可提现余额快照。
		field.Float("balance_before").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Float("balance_after").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		// payment_method: 支付方式（alipay/wechat）。
		field.String("payment_method").
			MaxLen(20).
			NotEmpty(),
		// payment_account: 收款账号（文本）。
		field.String("payment_account").
			MaxLen(255).
			NotEmpty(),
		// status: 提现状态。pending/settled/rejected/cancelled。
		field.String("status").
			MaxLen(20).
			Default(domain.WithdrawalStatusPending),
		// processed_by_user_id: 处理（审核/打款）管理员用户 ID。
		field.Int64("processed_by_user_id").
			Optional().
			Nillable(),
		// processed_at: 处理时间。
		field.Time("processed_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

// Indexes 定义提现单的查询索引。
func (Withdrawal) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_user_id"),
		index.Fields("owner_user_id", "status"),
		index.Fields("status"),
		index.Fields("processed_by_user_id"),
	}
}
