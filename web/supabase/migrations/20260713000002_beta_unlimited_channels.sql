-- Free-for-all beta: everyone gets Pro features, no payment. The alert-channel
-- limit is enforced by an RLS policy (the DB is the real gate for direct
-- PostgREST inserts), so lifting it in app code isn't enough — replace the
-- plan-limit insert policy with a plain owner-only one for the beta.
--
-- To end the beta and re-enable paid limits, re-apply the policy from
-- 20260705000002_channel_limit_policy.sql and set BETA_MODE off.

drop policy if exists "create own alert channels within plan limit" on public.alert_channels;

create policy "create own alert channels"
  on public.alert_channels for insert
  with check ((select auth.uid()) = user_id);
