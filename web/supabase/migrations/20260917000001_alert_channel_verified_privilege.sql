-- alert_channels.verified must not be client-writable.
--
-- The table carried a table-level `grant insert, update ... to authenticated`
-- and RLS policies that only check user_id, so any signed-in user could POST
-- straight to PostgREST with {"verified": true} and skip verification
-- entirely. A channel is what makes the product send a message, so that turned
-- any account into a sender of attacker-worded email, from our own SES domain,
-- to an address nobody confirmed. The alert title and body are submitter
-- controlled (repo_label and the check message), and nothing caps agents per
-- account.
--
-- `verified` means "a message was successfully delivered to this destination".
-- That is decided in Node, by sendToChannel actually succeeding, so it cannot
-- be enforced by a policy or by a function the user is able to call directly:
-- either would be equally spoofable. The privilege is therefore removed from
-- `authenticated` outright, and the application flips the flag with the
-- service role only after a real send returns.
--
-- SELECT and DELETE stay table-level: reading and removing your own channels
-- is already fully governed by the existing RLS policies.

revoke insert, update on public.alert_channels from authenticated;

-- Creating a channel stays a client operation, so the RLS insert policy (and
-- with it the plan-limit gate) remains the thing that governs it. `verified`
-- is simply not in the list, so it keeps its `default false`.
grant insert (user_id, type, config) on public.alert_channels to authenticated;

-- Editing a channel's destination is allowed; re-verification is triggered by
-- the application, which flips `verified` back through the service role.
grant update (config) on public.alert_channels to authenticated;
