-- 231_share_schema.sql
-- 账号托管共享 + 平台分账 数据层迁移（PostgreSQL 方言）
--
-- 内容：
--   1. 既有 3 张表增量加列（accounts / users / usage_logs）
--   2. 新建 6 张表（share_policies / owner_ledgers / settlements / withdrawals
--      / account_share_rooms / account_share_memberships）
--   3. 全局默认分账策略种子（owner_share_ratio = 0.7）
--   4. owner_ledgers (ref_type, ref_id) 幂等部分唯一索引
--   5. share_policies.owner_share_ratio ∈ [0,1] CHECK 约束（内联在 CREATE TABLE）
--   6. withdrawals 每号主最多一笔 pending 的部分唯一索引
--
-- 全部语句幂等，可重复执行。

-- ============================================================================
-- 1. 既有表增量加列（可平滑升级已有 sub2api 数据库）
-- ============================================================================

-- accounts：账号托管共享字段
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS owner_user_id BIGINT;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS share_mode VARCHAR(20) NOT NULL DEFAULT 'private';
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS share_status VARCHAR(20) NOT NULL DEFAULT 'pending';
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS share_policy_id BIGINT;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS account_level VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS max_seats INT NOT NULL DEFAULT 1;

CREATE INDEX IF NOT EXISTS idx_accounts_owner_user_id ON accounts(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_accounts_share_mode_status ON accounts(share_mode, share_status);

-- users：号主托管权限开关
ALTER TABLE users ADD COLUMN IF NOT EXISTS can_host_accounts BOOLEAN NOT NULL DEFAULT FALSE;

-- usage_logs：用量归属号主（分账归因）
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS owner_user_id BIGINT;
CREATE INDEX IF NOT EXISTS idx_usage_logs_owner_user_id ON usage_logs(owner_user_id);

-- ============================================================================
-- 2. 新建 6 张表
-- ============================================================================

-- 2.1 分账策略
CREATE TABLE IF NOT EXISTS share_policies (
    id                  BIGSERIAL PRIMARY KEY,
    scope_type          VARCHAR(20) NOT NULL,                              -- global/platform/account/group
    scope_id            BIGINT,                                            -- 作用域实体 ID（global 为 NULL）
    platform            VARCHAR(50),                                       -- 平台维度覆盖（NULL = 全部平台）
    owner_share_ratio   DECIMAL(10,4) NOT NULL CHECK (owner_share_ratio >= 0 AND owner_share_ratio <= 1),
    invite_share_ratio  DECIMAL(10,4) NOT NULL DEFAULT 0,
    version             INT NOT NULL DEFAULT 1,
    enabled             BOOLEAN NOT NULL DEFAULT TRUE,
    effective_at        TIMESTAMPTZ,
    created_by_admin_id BIGINT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_share_policies_scope ON share_policies(scope_type, scope_id);
CREATE INDEX IF NOT EXISTS idx_share_policies_platform ON share_policies(platform);
CREATE INDEX IF NOT EXISTS idx_share_policies_enabled ON share_policies(enabled);
CREATE INDEX IF NOT EXISTS idx_share_policies_deleted_at ON share_policies(deleted_at);

-- 2.2 收益账本
CREATE TABLE IF NOT EXISTS owner_ledgers (
    id             BIGSERIAL PRIMARY KEY,
    owner_user_id  BIGINT NOT NULL,
    direction      VARCHAR(20) NOT NULL,                                   -- credit/debit
    amount         DECIMAL(20,10) NOT NULL,
    balance_before DECIMAL(20,10) NOT NULL,
    balance_after  DECIMAL(20,10) NOT NULL,
    entry_type     VARCHAR(30) NOT NULL,                                   -- share_credit/settlement/withdrawal/adjustment/refund
    ref_type       VARCHAR(30),
    ref_id         BIGINT,
    metadata       JSONB,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_owner_ledgers_owner_user_id ON owner_ledgers(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_owner_ledgers_owner_created ON owner_ledgers(owner_user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_owner_ledgers_direction ON owner_ledgers(direction);
CREATE INDEX IF NOT EXISTS idx_owner_ledgers_entry_type ON owner_ledgers(entry_type);
CREATE INDEX IF NOT EXISTS idx_owner_ledgers_ref ON owner_ledgers(ref_type, ref_id);

-- 2.3 结算单
CREATE TABLE IF NOT EXISTS settlements (
    id            BIGSERIAL PRIMARY KEY,
    owner_user_id BIGINT NOT NULL,
    settlement_no VARCHAR(64) NOT NULL UNIQUE,
    period_start  TIMESTAMPTZ NOT NULL,
    period_end    TIMESTAMPTZ NOT NULL,
    gross_amount  DECIMAL(20,10) NOT NULL DEFAULT 0,
    fee_amount    DECIMAL(20,10) NOT NULL DEFAULT 0,
    net_amount    DECIMAL(20,10) NOT NULL DEFAULT 0,
    status        VARCHAR(20) NOT NULL DEFAULT 'pending',                  -- pending/settled/failed
    settled_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_settlements_owner_user_id ON settlements(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_settlements_owner_status ON settlements(owner_user_id, status);
CREATE INDEX IF NOT EXISTS idx_settlements_status ON settlements(status);
CREATE INDEX IF NOT EXISTS idx_settlements_period ON settlements(period_start, period_end);

-- 2.4 提现单
CREATE TABLE IF NOT EXISTS withdrawals (
    id                    BIGSERIAL PRIMARY KEY,
    owner_user_id         BIGINT NOT NULL,
    amount                DECIMAL(20,10) NOT NULL,
    fee_amount            DECIMAL(20,10) NOT NULL DEFAULT 0,
    total_deducted        DECIMAL(20,10) NOT NULL DEFAULT 0,
    balance_before        DECIMAL(20,10) NOT NULL,
    balance_after         DECIMAL(20,10) NOT NULL,
    payment_method        VARCHAR(20) NOT NULL,                            -- alipay/wechat
    payment_account       VARCHAR(255) NOT NULL,
    status                VARCHAR(20) NOT NULL DEFAULT 'pending',          -- pending/settled/rejected/cancelled
    processed_by_user_id  BIGINT,
    processed_at          TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_withdrawals_owner_user_id ON withdrawals(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_withdrawals_owner_status ON withdrawals(owner_user_id, status);
CREATE INDEX IF NOT EXISTS idx_withdrawals_status ON withdrawals(status);
CREATE INDEX IF NOT EXISTS idx_withdrawals_processed_by ON withdrawals(processed_by_user_id);

-- 2.5 广场房间
CREATE TABLE IF NOT EXISTS account_share_rooms (
    id                   BIGSERIAL PRIMARY KEY,
    account_id           BIGINT NOT NULL UNIQUE,                           -- 一个账号最多一个广场房间
    owner_user_id        BIGINT NOT NULL,
    name                 VARCHAR(100) NOT NULL,
    status               VARCHAR(20) NOT NULL DEFAULT 'active',            -- active/disabled/closed
    seat_limit           INT NOT NULL DEFAULT 5,
    rate_multiplier      DECIMAL(10,4) NOT NULL DEFAULT 1,
    allowed_models       JSONB,
    per_user_concurrency INT NOT NULL DEFAULT 1,
    queue_max            INT NOT NULL DEFAULT 20,
    version              INT NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_account_share_rooms_owner_user_id ON account_share_rooms(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_account_share_rooms_status ON account_share_rooms(status);

-- 2.6 广场会员（座位/排队）
CREATE TABLE IF NOT EXISTS account_share_memberships (
    id               BIGSERIAL PRIMARY KEY,
    room_id          BIGINT NOT NULL,
    account_id       BIGINT NOT NULL,
    consumer_user_id BIGINT NOT NULL,
    api_key_id       BIGINT,
    status           VARCHAR(20) NOT NULL DEFAULT 'active',                -- active/queued/ended/expired
    joined_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at         TIMESTAMPTZ,
    queue_expires_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_account_share_memberships_room_id ON account_share_memberships(room_id);
CREATE INDEX IF NOT EXISTS idx_account_share_memberships_account_id ON account_share_memberships(account_id);
CREATE INDEX IF NOT EXISTS idx_account_share_memberships_consumer ON account_share_memberships(consumer_user_id);
CREATE INDEX IF NOT EXISTS idx_account_share_memberships_api_key ON account_share_memberships(api_key_id);
CREATE INDEX IF NOT EXISTS idx_account_share_memberships_status ON account_share_memberships(status);
CREATE INDEX IF NOT EXISTS idx_account_share_memberships_queue ON account_share_memberships(queue_expires_at);

-- ============================================================================
-- 3. 全局默认分账策略种子（owner_share_ratio = 0.7）
-- ============================================================================

INSERT INTO share_policies (
    scope_type, scope_id, platform, owner_share_ratio, invite_share_ratio,
    version, enabled, effective_at, created_by_admin_id, created_at, updated_at
)
SELECT 'global', NULL, NULL, 0.7, 0, 1, TRUE, NULL, NULL, NOW(), NOW()
WHERE NOT EXISTS (
    SELECT 1 FROM share_policies WHERE scope_type = 'global' AND deleted_at IS NULL
);

-- ============================================================================
-- 4. owner_ledgers (ref_type, ref_id) 幂等部分唯一索引
--    仅对「同时非空」的 ref 建立唯一约束，避免 NULL 重复冲突；
--    保证同一业务对象（如同一 usage_log）只入账一次。
-- ============================================================================

CREATE UNIQUE INDEX IF NOT EXISTS owner_ledgers_ref_type_ref_id_uq
    ON owner_ledgers (ref_type, ref_id)
    WHERE ref_type IS NOT NULL AND ref_id IS NOT NULL;

-- ============================================================================
-- 5. withdrawals 每号主最多一笔 pending 的部分唯一索引
-- ============================================================================

CREATE UNIQUE INDEX IF NOT EXISTS withdrawals_owner_pending_uq
    ON withdrawals (owner_user_id)
    WHERE status = 'pending';
