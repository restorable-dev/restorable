-- The beta lifted the channel-limit policy (20260713000002), which was the
-- only caller of own_alert_channel_count(). Revoke the now-unused EXECUTE from
-- authenticated so the SECURITY DEFINER function isn't callable via RPC while
-- unused. Re-granting is required when the channel-limit policy is restored to
-- end the beta.
revoke execute on function public.own_alert_channel_count() from authenticated;
