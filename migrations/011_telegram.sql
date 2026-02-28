-- Folio Migration 011: Telegram Bot
-- Adds: telegram_users, bot_notifications tables

CREATE TABLE IF NOT EXISTS telegram_users (
    telegram_id  BIGINT PRIMARY KEY,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    username     TEXT DEFAULT '',
    display_name TEXT DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bot_notifications (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type         TEXT NOT NULL CHECK (type IN ('digest', 'watchlist_hit', 'system')),
    payload      JSONB NOT NULL DEFAULT '{}',
    delivered    BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_bot_notif_pending ON bot_notifications(delivered, created_at) WHERE delivered = false;
