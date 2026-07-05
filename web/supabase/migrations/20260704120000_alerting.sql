-- Alerting + stale detection (Phase 4).

-- ─── alert_channels ─────────────────────────────────────────────────────────
-- Where a user's alerts go. config shape depends on type:
--   telegram: {"chat_id": "123456"}
--   discord:  {"webhook_url": "https://discord.com/api/webhooks/…"}
--   ntfy:     {"topic": "my-topic", "server": "https://ntfy.sh" (optional)}
--   email:    {"to": "user@example.com"}
create table public.alert_channels (
  id         uuid primary key default gen_random_uuid(),
  user_id    uuid not null references public.profiles (id) on delete cascade,
  type       text not null check (type in ('telegram', 'discord', 'ntfy', 'email')),
  config     jsonb not null,
  -- Set true after a test message goes through. Only verified channels
  -- receive alerts.
  verified   boolean not null default false,
  created_at timestamptz not null default now()
);

create index alert_channels_user_id_idx on public.alert_channels (user_id);

alter table public.alert_channels enable row level security;

create policy "read own alert channels"
  on public.alert_channels for select
  using ((select auth.uid()) = user_id);

create policy "create own alert channels"
  on public.alert_channels for insert
  with check ((select auth.uid()) = user_id);

create policy "update own alert channels"
  on public.alert_channels for update
  using ((select auth.uid()) = user_id)
  with check ((select auth.uid()) = user_id);

create policy "delete own alert channels"
  on public.alert_channels for delete
  using ((select auth.uid()) = user_id);

-- ─── alerts ─────────────────────────────────────────────────────────────────
-- Incident model: one open ('firing') incident per (user, kind, subject).
-- Opening dispatches an alert; resolving dispatches a recovery notice.
-- Duplicate conditions while an incident is open are suppressed by the
-- partial unique index — dedup is enforced by the database, not by code.
create table public.alerts (
  id          uuid primary key default gen_random_uuid(),
  user_id     uuid not null references public.profiles (id) on delete cascade,
  kind        text not null check (kind in ('run_failure', 'stale_repo', 'agent_silent')),
  -- The repo or agent this incident is about.
  subject_id  uuid not null,
  run_id      uuid references public.test_runs (id) on delete set null,
  status      text not null default 'firing' check (status in ('firing', 'resolved')),
  message     text not null,
  fired_at    timestamptz not null default now(),
  resolved_at timestamptz
);

create unique index alerts_one_open_incident
  on public.alerts (user_id, kind, subject_id)
  where status = 'firing';

create index alerts_user_fired_idx on public.alerts (user_id, fired_at desc);

alter table public.alerts enable row level security;

create policy "read own alerts"
  on public.alerts for select
  using ((select auth.uid()) = user_id);
-- Writes are service-role only (runs endpoint + cron).

-- ─── staleness / silence knobs ──────────────────────────────────────────────
-- stale_after null = automatic: 1.5× the gap between the two most recent
-- runs (min 24h), once at least two runs exist (SPEC: schedule window × 1.5).
alter table public.repos
  add column stale_after interval;

-- Daemon agents heartbeat every 6h; opt out for cron-driven agents that
-- legitimately only check in when they run.
alter table public.agents
  add column alert_when_silent boolean not null default true;

-- ─── grants ─────────────────────────────────────────────────────────────────
grant select, insert, update, delete on public.alert_channels to authenticated;
grant select                         on public.alerts         to authenticated;
grant all on public.alert_channels, public.alerts to service_role;
