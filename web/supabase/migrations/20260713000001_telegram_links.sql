-- Telegram connect via deep link (replaces the getUpdates chat-ID lookup,
-- which leaked other users' chats across the shared bot).
--
-- Flow: the dashboard mints a one-time token and shows a t.me/<bot>?start=<token>
-- link. The user taps it; Telegram sends the bot `/start <token>`; the bot
-- webhook matches the token to this row and stamps the chat_id. The dashboard
-- then creates the verified alert_channel. Each user only ever learns their
-- own chat_id — nothing is shared.

create table public.telegram_links (
  token       text primary key,
  user_id     uuid not null references public.profiles (id) on delete cascade,
  chat_id     text,
  chat_name   text,
  created_at  timestamptz not null default now(),
  expires_at  timestamptz not null default now() + interval '15 minutes',
  consumed_at timestamptz
);

create index telegram_links_user_idx on public.telegram_links (user_id);

alter table public.telegram_links enable row level security;

-- The owner may read their own link (to poll for the chat_id landing) and
-- create one. The webhook (service role) stamps chat_id.
create policy "read own telegram link"
  on public.telegram_links for select
  using ((select auth.uid()) = user_id);

create policy "create own telegram link"
  on public.telegram_links for insert
  with check ((select auth.uid()) = user_id);

grant select, insert on public.telegram_links to authenticated;
grant all on public.telegram_links to service_role;
