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

// OwnerLedger 定义号主收益账本实体 schema。
//
// 收益账本记录号主每一笔收益变动（分成入账/结算/提现/调整等），
// 为只追加流水，通过 balance_before/balance_after 快照保证可审计。
type OwnerLedger struct {
	ent.Schema
}

// Annotations 指定数据库表名为 "owner_ledgers"。
func (OwnerLedger) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "owner_ledgers"},
	}
}

// Mixin 仅使用时间戳混入组件（账本为只追加，不做软删除）。
func (OwnerLedger) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

// Fields 定义收益账本实体的所有字段。
func (OwnerLedger) Fields() []ent.Field {
	return []ent.Field{
		// owner_user_id: 收益归属号主用户 ID。
		field.Int64("owner_user_id"),
		// direction: 流水方向。credit=入账；debit=出账。
		field.Enum("direction").
			Values(domain.LedgerDirectionCredit, domain.LedgerDirectionDebit).
			Comment("Ledger direction: credit/debit."),
		// amount: 本笔变动金额（正数）。
		field.Float("amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		// balance_before / balance_after: 变动前后可用余额快照。
		field.Float("balance_before").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Float("balance_after").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		// entry_type: 流水条目类型（share_credit/settlement/withdrawal/adjustment/refund）。
		field.String("entry_type").
			MaxLen(30).
			NotEmpty(),
		// ref_type / ref_id: 关联业务对象（如 usage_log/settlement/withdrawal），用于幂等。
		field.String("ref_type").
			MaxLen(30).
			Optional().
			Nillable(),
		field.Int64("ref_id").
			Optional().
			Nillable(),
		// metadata: 扩展元数据（JSONB）。
		field.JSON("metadata", map[string]any{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
	}
}

// Indexes 定义收益账本的查询索引。
func (OwnerLedger) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_user_id"),
		index.Fields("owner_user_id", "created_at"),
		index.Fields("direction"),
		index.Fields("entry_type"),
		// (ref_type, ref_id) 幂等唯一约束由迁移 SQL 以部分唯一索引实现。
		index.Fields("ref_type", "ref_id"),
	}
}
