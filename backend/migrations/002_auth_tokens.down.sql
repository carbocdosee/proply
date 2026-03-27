-- Rollback: 002_auth_tokens
DROP TABLE IF EXISTS auth_tokens;

ALTER TABLE job_queue DROP CONSTRAINT IF EXISTS job_queue_job_type_check;
ALTER TABLE job_queue ADD CONSTRAINT job_queue_job_type_check CHECK (job_type IN (
  'email_open_notify',
  'email_approved_notify',
  'email_client_approved',
  'email_payment_failed',
  'gdpr_hard_delete'
));
