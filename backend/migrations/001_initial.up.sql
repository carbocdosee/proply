-- Migration: 001_initial
-- Creates the base schema for Proply

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users (agency accounts)
CREATE TABLE users (
  id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email                TEXT NOT NULL UNIQUE,
  name                 TEXT NOT NULL DEFAULT '',
  password_hash        TEXT,                     -- NULL if Google OAuth only
  google_id            TEXT UNIQUE,
  email_verified_at    TIMESTAMPTZ,
  plan                 TEXT NOT NULL DEFAULT 'free' CHECK (plan IN ('free', 'pro', 'team')),
  language             TEXT NOT NULL DEFAULT 'en' CHECK (language IN ('en', 'ru')),
  logo_url             TEXT,
  primary_color        TEXT DEFAULT '#6366F1',
  accent_color         TEXT DEFAULT '#F59E0B',
  hide_proply_footer   BOOLEAN NOT NULL DEFAULT FALSE,
  data_retention_months INT NOT NULL DEFAULT 24,
  stripe_customer_id   TEXT UNIQUE,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at           TIMESTAMPTZ            -- soft delete for GDPR flow
);

CREATE INDEX users_email_idx ON users(email) WHERE deleted_at IS NULL;

-- Subscriptions (billing state)
CREATE TABLE subscriptions (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id            UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider           TEXT NOT NULL CHECK (provider IN ('stripe', 'paddle')),
  external_id        TEXT NOT NULL UNIQUE,   -- stripe/paddle subscription ID
  plan               TEXT NOT NULL CHECK (plan IN ('pro', 'team')),
  status             TEXT NOT NULL CHECK (status IN ('active', 'cancelled', 'past_due')),
  current_period_end TIMESTAMPTZ NOT NULL,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Proposals (commercial proposal documents)
CREATE TABLE proposals (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title           TEXT NOT NULL DEFAULT 'Без названия',
  client_name     TEXT NOT NULL DEFAULT '',
  client_email    TEXT,                    -- captured on approval
  status          TEXT NOT NULL DEFAULT 'draft'
                  CHECK (status IN ('draft', 'sent', 'opened', 'approved', 'rejected')),
  slug            TEXT,                    -- NULL while draft; set on publish
  slug_active     BOOLEAN NOT NULL DEFAULT TRUE,
  password_hash   TEXT,                    -- optional password protection
  blocks          JSONB NOT NULL DEFAULT '[]',
  template_id     TEXT,
  -- tracking summary (denormalized for fast list queries)
  first_opened_at TIMESTAMPTZ,
  last_opened_at  TIMESTAMPTZ,
  open_count      INT NOT NULL DEFAULT 0,
  approved_at     TIMESTAMPTZ,
  -- timestamps
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at      TIMESTAMPTZ
);

CREATE UNIQUE INDEX proposals_slug_idx ON proposals(slug) WHERE slug IS NOT NULL;
CREATE INDEX proposals_user_status_idx ON proposals(user_id, status) WHERE deleted_at IS NULL;
CREATE INDEX proposals_user_created_idx ON proposals(user_id, created_at DESC) WHERE deleted_at IS NULL;

-- Tracking events (view analytics)
CREATE TABLE tracking_events (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  proposal_id  UUID NOT NULL REFERENCES proposals(id) ON DELETE CASCADE,
  event_type   TEXT NOT NULL CHECK (event_type IN ('open', 'block_time', 'approve')),
  block_id     UUID,                        -- for event_type = 'block_time'
  duration_ms  INT,                         -- for event_type = 'block_time'
  country      TEXT,                        -- 2-letter ISO country code
  user_agent   TEXT NOT NULL DEFAULT '',
  fingerprint  TEXT NOT NULL DEFAULT '',    -- SHA-256(IP + UA)[:16], no raw IP stored
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX tracking_events_proposal_idx ON tracking_events(proposal_id, event_type);
CREATE INDEX tracking_events_fingerprint_idx ON tracking_events(proposal_id, fingerprint, created_at);

-- Background job queue
CREATE TABLE job_queue (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  job_type     TEXT NOT NULL CHECK (job_type IN (
                 'email_open_notify',
                 'email_approved_notify',
                 'email_client_approved',
                 'email_payment_failed',
                 'gdpr_hard_delete'
               )),
  payload      JSONB NOT NULL,
  status       TEXT NOT NULL DEFAULT 'pending'
               CHECK (status IN ('pending', 'processing', 'done', 'failed')),
  attempts     INT NOT NULL DEFAULT 0,
  max_attempts INT NOT NULL DEFAULT 3,
  scheduled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  processed_at TIMESTAMPTZ,
  error        TEXT,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX job_queue_pending_idx ON job_queue(status, scheduled_at) WHERE status = 'pending';

-- Processed webhooks (idempotency for Stripe/Paddle)
CREATE TABLE processed_webhooks (
  event_id    TEXT PRIMARY KEY,
  provider    TEXT NOT NULL CHECK (provider IN ('stripe', 'paddle')),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
