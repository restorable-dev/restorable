import { authenticateAgent, unauthorized } from "@/lib/api/agent-auth";
import { runRequestSchema } from "@/lib/api/schemas";
import { createAdminClient } from "@/lib/supabase/admin";

// POST /api/v1/runs
// Accepts one run result from an agent. Creates the repo row on first sight
// of its fingerprint. Only pass/fail metadata ever arrives here — the agent
// scrubs everything else before it leaves the user's machine.
export async function POST(request: Request) {
  const admin = createAdminClient();
  const agent = await authenticateAgent(admin, request);
  if (!agent) {
    return unauthorized();
  }

  let body: unknown;
  try {
    body = await request.json();
  } catch {
    return Response.json({ error: "request body must be JSON" }, { status: 400 });
  }
  const parsed = runRequestSchema.safeParse(body);
  if (!parsed.success) {
    return Response.json(
      { error: "invalid request", details: parsed.error.flatten() },
      { status: 400 },
    );
  }
  const run = parsed.data;

  // First run for this fingerprint creates the repo; later runs keep the
  // user's chosen label (upsert ignores duplicates instead of overwriting).
  const { error: upsertError } = await admin
    .from("repos")
    .upsert(
      {
        user_id: agent.user_id,
        fingerprint: run.repo_fingerprint,
        label: run.repo_label,
      },
      { onConflict: "user_id,fingerprint", ignoreDuplicates: true },
    );
  if (upsertError) {
    return Response.json({ error: "could not record repo" }, { status: 500 });
  }
  const { data: repo, error: repoError } = await admin
    .from("repos")
    .select("id")
    .eq("user_id", agent.user_id)
    .eq("fingerprint", run.repo_fingerprint)
    .single();
  if (repoError || !repo) {
    return Response.json({ error: "could not record repo" }, { status: 500 });
  }

  const { data: inserted, error: runError } = await admin
    .from("test_runs")
    .insert({
      user_id: agent.user_id,
      agent_id: agent.id,
      repo_id: repo.id,
      snapshot_id: run.snapshot_id ?? null,
      status: run.status,
      error: run.error ?? null,
      results: run.checks,
      agent_version: run.agent_version,
      started_at: run.started_at,
      finished_at: run.finished_at,
    })
    .select("id")
    .single();
  if (runError || !inserted) {
    return Response.json({ error: "could not record run" }, { status: 500 });
  }

  await admin
    .from("agents")
    .update({ last_seen: new Date().toISOString(), agent_version: run.agent_version })
    .eq("id", agent.id);

  return Response.json({ run_id: inserted.id }, { status: 201 });
}
