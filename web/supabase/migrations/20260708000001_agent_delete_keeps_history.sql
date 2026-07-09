-- Deleting an agent (e.g. to rotate a leaked key) must not erase the run
-- history it reported. History belongs to the repo; the agent is just the
-- messenger. Runs keep a nullable pointer that clears on agent deletion.

alter table public.test_runs
  alter column agent_id drop not null;

alter table public.test_runs
  drop constraint test_runs_agent_id_fkey;

alter table public.test_runs
  add constraint test_runs_agent_id_fkey
  foreign key (agent_id) references public.agents (id) on delete set null;
