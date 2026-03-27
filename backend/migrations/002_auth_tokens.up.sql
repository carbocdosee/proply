-- Migration: 002_auth_tokens
-- Adds auth_tokens table for magic links and email verification.
-- Also updates job_queue to allow new job types.

CREATE TABLE auth_tokens (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  token_hash  TEXT NOT NULL UNIQUE,        -- SHA-256(raw_token), never store raw
  type        TEXT NOT NULL CHECK (type IN ('magic_link', 'email_verification')),
  user_id     UUID REFERENCES users(id) ON DELETE CASCADE, -- NULL for magic links to unknown emails
  email       TEXT NOT NULL,               -- target email (used for magic links to new users)
  expires_at  TIMESTAMPTZ NOT NULL,
  used_at     TIMESTAMPTZ,                 -- NULL = not yet used
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX auth_tokens_hash_idx ON auth_tokens(token_hash);
CREATE INDEX auth_tokens_user_idx ON auth_tokens(user_id) WHERE user_id IS NOT NULL;

-- Add email_verification and magic_link job types to job_queue check constraint.
-- PostgreSQL requires dropping and recreating the constraint.
ALTER TABLE job_queue DROP CONSTRAINT IF EXISTS job_queue_job_type_check;
ALTER TABLE job_queue ADD CONSTRAINT job_queue_job_type_check CHECK (job_type IN (
  'email_open_notify',
  'email_approved_notify',
  'email_client_approved',
  'email_payment_failed',
  'email_verification',
  'email_magic_link',
  'gdpr_hard_delete'
));

-- Add email_verified_at to users if not already present (idempotent).
-- Column already exists from migration 001, this is a no-op comment.
-- users.email_verified_at TIMESTAMPTZ already declared.
