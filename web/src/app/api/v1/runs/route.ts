import { waitUntil } from "@vercel/functions";

import { authenticateAgent, unauthorized } from "@/lib/api/agent-auth";
import { runRequestSchema, type RunRequest } from "@/lib/api/schemas";
import { openIncident, resolveIncident } from "@/lib/alerts/incidents";
import { getLimits } from "@/lib/billing/entitlements";
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

  // Ingestion cap per agent: an hourly ceiling far above any sane test
  // cadence, so a buggy or hostile agent can't fill the database.
  const hourAgo = new Date(Date.now() - 60 * 60 * 1000).toISOString();
  const { count: recentRuns } = await admin
    .from("test_runs")
    .select("id", { count: "exact", head: true })
    .eq("agent_id", agent.id)
    .gte("created_at", hourAgo);
  const runsPerHourLimit = Number(process.env.RUNS_PER_HOUR_LIMIT ?? 60);
  if ((recentRuns ?? 0) >= runsPerHourLimit) {
    return Response.json(
      { error: `rate limited: this agent submitted ${recentRuns} runs in the last hour` },
      { status: 429 },
    );
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

  // First run for this fingerprint creates the repo — gated by the plan's
  // repo limit. Existing repos always keep working: limits cap growth, they
  // never break what's already reporting.
  let { data: repo } = await admin
    .from("repos")
    .select("id")
    .eq("user_id", agent.user_id)
    .eq("fingerprint", run.repo_fingerprint)
    .maybeSingle();
  if (!repo) {
    const limits = await getLimits(admin, agent.user_id);
    if (limits.maxRepos != null) {
      const { count } = await admin
        .from("repos")
        .select("id", { count: "exact", head: true })
        .eq("user_id", agent.user_id);
      if ((count ?? 0) >= limits.maxRepos) {
        return Response.json(
          {
            error: `plan limit: the free plan monitors ${limits.maxRepos} repository — upgrade to Pro for unlimited repos`,
          },
          { status: 403 },
        );
      }
    }
    const { data: created, error: insertError } = await admin
      .from("repos")
      .insert({
        user_id: agent.user_id,
        fingerprint: run.repo_fingerprint,
        label: run.repo_label,
      })
      .select("id")
      .single();
    if (insertError || !created) {
      return Response.json({ error: "could not record repo" }, { status: 500 });
    }
    repo = created;
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
  // Dispatch runs AFTER the response (waitUntil): a slow Discord webhook must
  // not stall — or time out — the agent's submission.
  const appUrl = process.env.NEXT_PUBLIC_APP_URL ?? "";
  const runLink = appUrl ? `\n${appUrl}/dashboard/runs/${inserted.id}` : "";
  const incident = { userId: agent.user_id, kind: "run_failure" as const, subjectId: repo.id };
  const runId = inserted.id;
  waitUntil(
    (async () => {
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
          runId,
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
    })().catch((err) => console.error("deferred alert dispatch failed:", err)),
  );

  return Response.json({ run_id: inserted.id }, { status: 201 });
}
