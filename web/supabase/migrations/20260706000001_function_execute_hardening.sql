-- Hardening per security advisor: SECURITY DEFINER functions must not be
-- callable via the public RPC surface by roles that don't need them.

-- Trigger-only function: nobody calls it directly; triggers fire it as the
-- table owner regardless of EXECUTE grants.
revoke execute on function public.handle_new_user() from public, anon, authenticated;

-- Policy helper: RLS insert checks evaluate it as the inserting role, so
-- authenticated keeps EXECUTE. It leaks nothing (returns only the caller's
-- own channel count), but anon has no business calling it.
revoke execute on function public.own_alert_channel_count() from public, anon;
