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

// AccountShareRoom 定义账号广场房间实体 schema。
//
// 广场房间将某个托管账号开放给平台用户，支持座位限制、排队与模型白名单。
type AccountShareRoom struct {
	ent.Schema
}

// Annotations 指定数据库表名为 "account_share_rooms"。
func (AccountShareRoom) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "account_share_rooms"},
	}
}

// Mixin 使用时间戳混入组件（房间生命周期由 status 字段追踪）。
func (AccountShareRoom) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

// Fields 定义广场房间实体的所有字段。
func (AccountShareRoom) Fields() []ent.Field {
	return []ent.Field{
		// account_id: 关联的托管账号 ID。一个账号最多一个广场房间。
		field.Int64("account_id").
			Unique(),
		// owner_user_id: 房间归属号主用户 ID。
		field.Int64("owner_user_id"),
		// name: 房间展示名称。
		field.String("name").
			MaxLen(100).
			NotEmpty(),
		// status: 房间状态。active/disabled/closed。
		field.String("status").
			MaxLen(20).
			Default(domain.RoomStatusActive),
		// seat_limit: 房间座位数上限。
		field.Int("seat_limit").
			Default(5),
		// rate_multiplier: 房间计费倍率。
		field.Float("rate_multiplier").
			SchemaType(map[string]string{dialect.Postgres: "decimal(10,4)"}).
			Default(1),
		// allowed_models: 允许调用的模型白名单（JSON 数组；NULL 表示不限制）。
		field.JSON("allowed_models", []string{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		// per_user_concurrency: 每用户并发数上限。
		field.Int("per_user_concurrency").
			Default(1),
		// queue_max: 排队队列长度上限。
		field.Int("queue_max").
			Default(20),
		// version: 房间配置版本号，用于乐观锁。
		field.Int("version").
			Default(0),
	}
}

// Indexes 定义广场房间的查询索引。
func (AccountShareRoom) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_user_id"),
		index.Fields("status"),
	}
}
