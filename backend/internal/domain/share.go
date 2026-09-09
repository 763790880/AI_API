// Package domain 定义业务领域常量。
// 本文件集中定义「账号托管共享 + 平台分账」相关的枚举字符串常量，
// 供 Ent schema 默认值、service/handler 状态流转与审计字段统一引用，
// 避免在各层散落魔法字符串。
package domain

// Account 共享模式常量（Account.share_mode）
const (
	ShareModePrivate = "private" // 私有：仅号主本人可用
	ShareModePublic  = "public"  // 公开：平台内按分组/策略共享调用
	ShareModePlaza   = "plaza"   // 广场：进入账号广场房间（预约/排队）
)

// Account 共享审核状态常量（Account.share_status）
const (
	ShareStatusPending  = "pending"  // 待审核
	ShareStatusApproved = "approved" // 审核通过
	ShareStatusRejected = "rejected" // 审核拒绝
)

// SharePolicy 作用域类型常量（SharePolicy.scope_type）
const (
	SharePolicyScopeGlobal   = "global"   // 全局默认策略
	SharePolicyScopePlatform = "platform" // 平台维度覆盖
	SharePolicyScopeAccount  = "account"  // 账号维度覆盖
	SharePolicyScopeGroup    = "group"    // 分组维度覆盖
)

// OwnerLedger 流水方向常量（OwnerLedger.direction）
const (
	LedgerDirectionCredit = "credit" // 入账（收益增加）
	LedgerDirectionDebit  = "debit"  // 出账（收益减少/冻结）
)

// OwnerLedger 条目类型常量（OwnerLedger.entry_type）
const (
	LedgerEntryTypeShareCredit        = "share_credit"        // 共享调用分成入账（credit，增加待结算 pending）
	LedgerEntryTypeSettlementPayout   = "settlement_payout"   // 结算打款（debit，pending 转 available）
	LedgerEntryTypeWithdrawalHold     = "withdrawal_hold"     // 提现冻结（debit，扣减 available）
	LedgerEntryTypeWithdrawalRelease  = "withdrawal_release"  // 提现解冻（credit，Reject/Cancel 时返还 available）
	LedgerEntryTypeSettlement         = "settlement"          // 兼容旧语义（保留）
	LedgerEntryTypeWithdrawal         = "withdrawal"          // 兼容旧语义（保留）
	LedgerEntryTypeAdjustment         = "adjustment"          // 人工调整
	LedgerEntryTypeRefund             = "refund"              // 退款冲正
)

// Settlement 结算单状态常量（Settlement.status）
const (
	SettlementStatusPending = "pending" // 待结算
	SettlementStatusSettled = "settled" // 已结算
	SettlementStatusFailed  = "failed"  // 结算失败
)

// Withdrawal 提现单状态常量（Withdrawal.status）
const (
	WithdrawalStatusPending   = "pending"   // 待审核
	WithdrawalStatusSettled   = "settled"   // 已打款
	WithdrawalStatusRejected  = "rejected"  // 已拒绝
	WithdrawalStatusCancelled = "cancelled" // 已取消
)

// AccountShareRoom 广场房间状态常量（AccountShareRoom.status）
const (
	RoomStatusActive   = "active"   // 运营中
	RoomStatusDisabled = "disabled" // 已停用
	RoomStatusClosed   = "closed"   // 已关闭
)

// AccountShareMembership 广场会员状态常量（AccountShareMembership.status）
const (
	MembershipStatusActive  = "active"  // 使用中
	MembershipStatusQueued  = "queued"  // 排队中
	MembershipStatusEnded   = "ended"   // 已结束
	MembershipStatusExpired = "expired" // 已过期
)

// Withdrawal 提现支付方式常量（Withdrawal.payment_method）
const (
	WithdrawalMethodAlipay = "alipay" // 支付宝
	WithdrawalMethodWechat = "wechat" // 微信
)
