-- Account sharing is opt-in per customer. Existing and new users are disabled.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS account_share_enabled BOOLEAN NOT NULL DEFAULT FALSE;

