-- Verified recovery time: how long the restic restore itself took, separate
-- from verification. Reported by agents >= v0.1.0; null for older runs and
-- for runs that failed before the restore completed.
alter table public.test_runs
  add column restore_duration_ms bigint
  check (restore_duration_ms is null or restore_duration_ms >= 0);
