-- stripe_events is the Stripe webhook idempotency ledger. It is written ONLY
-- by the webhook handler via the service-role key (which bypasses RLS); no
-- client (anon or authenticated) should ever read or write it. RLS was already
-- enabled with no policy, which correctly denies all client access — but an
-- implicit deny is indistinguishable from a forgotten policy to a reviewer or
-- the Supabase linter (advisor: rls_enabled_no_policy).
--
-- This makes the deny-all explicit. It changes no behavior: service_role still
-- bypasses RLS, and anon/authenticated remain fully denied.
create policy "stripe_events is service-role only (no client access)"
  on public.stripe_events
  for all
  to authenticated, anon
  using (false)
  with check (false);
