-- Core schema for the Restorable control plane (Phase 3).
-- Every table gets RLS at creation time. Users only ever see their own rows;
-- agents never get database credentials at all — they talk to /api/v1, which
-- authenticates them by hashed API key and uses the service role server-side.

-- ─── profiles ───────────────────────────────────────────────────────────────
create table public.profiles (
  id         uuid primary key references auth.users (id) on delete cascade,
  email      text not null,
  created_at timestamptz not null default now()
);

alter table public.profiles enable row level security;

create policy "read own profile"
  on public.profiles for select
  using ((select auth.uid()) = id);

-- Created automatically when an auth user is created.
create function public.handle_new_user()
returns trigger
language plpgsql
security definer
set search_path = ''
as $$
begin
  insert into public.profiles (id, email)
  values (new.id, coalesce(new.email, ''));
  return new;
end;
$$;

create trigger on_auth_user_created
  after insert on auth.users
  for each row execute function public.handle_new_user();

-- ─── agents ─────────────────────────────────────────────────────────────────
create table public.agents (
  id            uuid primary key default gen_random_uuid(),
  user_id       uuid not null references public.profiles (id) on delete cascade,
  name          text not null,
  -- sha256 hex of the API key. The key itself is shown once and never stored.
  key_hash      text not null unique,
  agent_version text,
  last_seen     timestamptz,
  created_at    timestamptz not null default now()
);

create index agents_user_id_idx on public.agents (user_id);

alter table public.agents enable row level security;

create policy "read own agents"
  on public.agents for select
  using ((select auth.uid()) = user_id);

create policy "rename own agents"
  on public.agents for update
  using ((select auth.uid()) = user_id)
  with check ((select auth.uid()) = user_id);

create policy "delete own agents"
  on public.agents for delete
  using ((select auth.uid()) = user_id);
-- No insert policy: agents are created by the registration endpoint
-- (service role) when a token is exchanged.

-- ─── registration_tokens ────────────────────────────────────────────────────
-- One-time tokens minted in the dashboard, exchanged by `restorable register`
-- for an agent API key. Only the sha256 hash is stored.
create table public.registration_tokens (
  id         uuid primary key default gen_random_uuid(),
  user_id    uuid not null references public.profiles (id) on delete cascade,
  token_hash text not null unique,
  created_at timestamptz not null default now(),
  expires_at timestamptz not null default now() + interval '1 hour',
  used_at    timestamptz
);

create index registration_tokens_user_id_idx on public.registration_tokens (user_id);

alter table public.registration_tokens enable row level security;

create policy "read own registration tokens"
  on public.registration_tokens for select
  using ((select auth.uid()) = user_id);

create policy "mint own registration tokens"
  on public.registration_tokens for insert
  with check ((select auth.uid()) = user_id);
-- No update/delete policies: consumption (used_at) is set by the service role.

-- ─── repos ──────────────────────────────────────────────────────────────────
-- A repo row is created the first time a run for its fingerprint arrives.
create table public.repos (
  id          uuid primary key default gen_random_uuid(),
  user_id     uuid not null references public.profiles (id) on delete cascade,
  -- sha256 hex of the credential-scrubbed repository string, computed
  -- agent-side. The control plane never needs the location itself.
  fingerprint text not null,
  label       text not null,
  created_at  timestamptz not null default now(),
  unique (user_id, fingerprint)
);

create index repos_user_id_idx on public.repos (user_id);

alter table public.repos enable row level security;

create policy "read own repos"
  on public.repos for select
  using ((select auth.uid()) = user_id);

create policy "relabel own repos"
  on public.repos for update
  using ((select auth.uid()) = user_id)
  with check ((select auth.uid()) = user_id);
-- No insert policy: repo rows are created by the runs endpoint (service role).

-- ─── test_runs ──────────────────────────────────────────────────────────────
create table public.test_runs (
  id            uuid primary key default gen_random_uuid(),
  user_id       uuid not null references public.profiles (id) on delete cascade,
  agent_id      uuid not null references public.agents (id) on delete cascade,
  repo_id       uuid not null references public.repos (id) on delete cascade,
  snapshot_id   text,
  status        text not null check (status in ('pass', 'fail', 'error')),
  error         text,
  -- Per-check results: [{recipe, type, status, message, duration_ms}]
  results       jsonb not null default '[]'::jsonb,
  agent_version text,
  started_at    timestamptz not null,
  finished_at   timestamptz not null,
  created_at    timestamptz not null default now()
);

create index test_runs_user_created_idx on public.test_runs (user_id, created_at desc);
create index test_runs_repo_created_idx on public.test_runs (repo_id, created_at desc);

alter table public.test_runs enable row level security;

create policy "read own test runs"
  on public.test_runs for select
  using ((select auth.uid()) = user_id);
-- No insert/update/delete policies: runs arrive only via /api/v1/runs
-- (service role).

-- ─── grants ─────────────────────────────────────────────────────────────────
-- Explicit least-privilege table grants (RLS then restricts rows). The anon
-- role gets nothing; agents talk to /api/v1, which uses the service role.
grant select                 on public.profiles            to authenticated;
grant select, update, delete on public.agents              to authenticated;
grant select, insert         on public.registration_tokens to authenticated;
grant select, update         on public.repos               to authenticated;
grant select                 on public.test_runs           to authenticated;

grant all on public.profiles, public.agents, public.registration_tokens,
             public.repos, public.test_runs to service_role;
