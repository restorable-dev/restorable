-- Billing + plan gating (Phase 5).

-- ─── subscriptions ──────────────────────────────────────────────────────────
-- One row per user, created on first checkout. plan is what entitlements
-- read; status tracks Stripe's view. Users with no row are on the free plan.
create table public.subscriptions (
  user_id                uuid primary key references public.profiles (id) on delete cascade,
  stripe_customer_id     text not null unique,
  stripe_subscription_id text unique,
  plan                   text not null default 'free' check (plan in ('free', 'pro')),
  status                 text not null default 'inactive',
  current_period_end     timestamptz,
  updated_at             timestamptz not null default now()
);

alter table public.subscriptions enable row level security;

create policy "read own subscription"
  on public.subscriptions for select
  using ((select auth.uid()) = user_id);
-- Writes come only from the Stripe webhook handler (service role).

-- ─── stripe_events ──────────────────────────────────────────────────────────
-- Webhook idempotency ledger: an event id is processed exactly once, enforced
-- by the primary key. Replays insert-conflict and are skipped.
create table public.stripe_events (
  id           text primary key,
  type         text not null,
  processed_at timestamptz not null default now()
);

alter table public.stripe_events enable row level security;
-- No policies: service-role only.

-- ─── grants ─────────────────────────────────────────────────────────────────
grant select on public.subscriptions to authenticated;
grant all on public.subscriptions, public.stripe_events to service_role;
