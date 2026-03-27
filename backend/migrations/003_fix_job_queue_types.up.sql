-- Migration: 003_fix_job_queue_types
-- Add email_magic_link and email_verification to job_queue job_type constraint.
-- The original constraint in 001_initial was missing these two types,
-- causing INSERT failures when sending magic links or verification emails.

ALTER TABLE job_queue DROP CONSTRAINT IF EXISTS job_queue_job_type_check;

ALTER TABLE job_queue ADD CONSTRAINT job_queue_job_type_check CHECK (job_type IN (
  'email_open_notify',
  'email_approved_notify',
  'email_client_approved',
  'email_payment_failed',
  'email_magic_link',
  'email_verification',
  'gdpr_hard_delete'
));
