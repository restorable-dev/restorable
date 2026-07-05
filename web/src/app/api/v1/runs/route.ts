import { authenticateAgent, unauthorized } from "@/lib/api/agent-auth";
import { runRequestSchema, type RunRequest } from "@/lib/api/schemas";
import { openIncident, resolveIncident } from "@/lib/alerts/incidents";
import { createAdminClient } from "@/lib/supabase/admin";

// firstProblem summarizes what went wrong for the alert body.
function firstProblem(run: RunRequest): string {
  const failed = run.checks.find((c) => c.status !== "pass");
  if (failed) {
    return `${failed.recipe}/${failed.type}: ${failed.message || failed.status}`;
  }
  return run.error ?? "verification failed";
}

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

  // Alerting. One incident per repo: first failure alerts, repeats are
  // suppressed by the incident index, the next pass sends a recovery notice.
  const appUrl = process.env.NEXT_PUBLIC_APP_URL ?? "";
  const runLink = appUrl ? `\n${appUrl}/dashboard/runs/${inserted.id}` : "";
  const incident = { userId: agent.user_id, kind: "run_failure" as const, subjectId: repo.id };
  if (run.status === "pass") {
    await resolveIncident(admin, incident, {
      level: "recovery",
      title: `Restore tests passing again — ${run.repo_label}`,
      body: `Snapshot ${run.snapshot_id?.slice(0, 8) ?? "?"} verified.${runLink}`,
    });
    // A fresh successful test also clears any staleness incident.
    await resolveIncident(
      admin,
      { ...incident, kind: "stale_repo" },
      {
        level: "recovery",
        title: `Backup tests running again — ${run.repo_label}`,
        body: `A successful restore test just completed.${runLink}`,
      },
    );
  } else {
    await openIncident(
      admin,
      incident,
      {
        level: "failure",
        title: `Restore test failed — ${run.repo_label}`,
        body: `${firstProblem(run)}${runLink}`,
      },
      inserted.id,
    );
  }
  // Any submission proves the agent is alive.
  await resolveIncident(
    admin,
    { userId: agent.user_id, kind: "agent_silent", subjectId: agent.id },
    {
      level: "recovery",
      title: "Agent is back online",
      body: "The agent just reported a test run.",
    },
  );

  return Response.json({ run_id: inserted.id }, { status: 201 });
}
