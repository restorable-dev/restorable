-- Enforce the alert-channel plan limit in the database itself, not just in
-- application code: alert_channels is directly insertable via PostgREST, so
-- the RLS insert policy is the real gate. Free (or no subscription row) = 1
-- channel; an active/trialing/past_due pro subscription lifts the cap.
--
-- The count runs in a SECURITY DEFINER function: a policy subquerying its
-- own table would recurse into itself (42P17).

create function public.own_alert_channel_count()
returns bigint
language sql
security definer
set search_path = ''
as $$
  select count(*) from public.alert_channels
  where user_id = (select auth.uid());
$$;

revoke all on function public.own_alert_channel_count() from public;
grant execute on function public.own_alert_channel_count() to authenticated;

drop policy "create own alert channels" on public.alert_channels;

create policy "create own alert channels within plan limit"
  on public.alert_channels for insert
  with check (
    (select auth.uid()) = user_id
    and public.own_alert_channel_count() < coalesce(
      (
        select case
          when s.plan = 'pro' and s.status in ('active', 'trialing', 'past_due')
            then 1000000
          else 1
        end
        from public.subscriptions s
        where s.user_id = (select auth.uid())
      ),
      1
    )
  );
