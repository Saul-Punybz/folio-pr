-- Phase 3: Daily briefs, keyword alerts, and alert matches.

CREATE TABLE briefs (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    date          DATE UNIQUE NOT NULL,
    summary       TEXT NOT NULL,
    top_tags      JSONB DEFAULT '[]',
    article_count INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE alerts (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    keywords   JSONB NOT NULL DEFAULT '[]',
    regions    JSONB DEFAULT '[]',
    active     BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_alerts_user_id ON alerts(user_id);

CREATE TABLE alert_matches (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    alert_id   UUID NOT NULL REFERENCES alerts(id) ON DELETE CASCADE,
    article_id UUID NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    seen       BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_alert_matches_alert_id ON alert_matches(alert_id);
CREATE INDEX idx_alert_matches_unseen ON alert_matches(seen) WHERE seen = false;
