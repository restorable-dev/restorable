import { openIncident, resolveIncident } from "@/lib/alerts/incidents";
import { isStale, parsePgInterval, staleAfterMs } from "@/lib/alerts/staleness";
import { createAdminClient } from "@/lib/supabase/admin";

// GET /api/cron/alerts — hourly (vercel.json). Detects the meta-failure:
// backups whose tests have gone quiet, and agents that stopped checking in.
// Guarded by CRON_SECRET (Vercel sends it as a Bearer token).

export const dynamic = "force-dynamic";

const AGENT_SILENT_MS = 24 * 60 * 60 * 1000;

function ago(ms: number): string {
  const hours = Math.round(ms / 3_600_000);
  if (hours < 48) {
    return `${hours}h ago`;
  }
  return `${Math.round(hours / 24)} days ago`;
}

export async function GET(request: Request) {
  const secret = process.env.CRON_SECRET;
  if (!secret || request.headers.get("authorization") !== `Bearer ${secret}`) {
    return Response.json({ error: "unauthorized" }, { status: 401 });
  }

  const admin = createAdminClient();
  const now = new Date();
  const summary = { stale_opened: 0, stale_resolved: 0, silent_opened: 0, silent_resolved: 0 };

  // ── stale repos ───────────────────────────────────────────────────────────
  const { data: repos } = await admin
    .from("repos")
    .select("id, user_id, label, stale_after");
  for (const repo of repos ?? []) {
    const { data: recent } = await admin
      .from("test_runs")
      .select("finished_at")
      .eq("repo_id", repo.id)
      .eq("status", "pass")
      .order("finished_at", { ascending: false })
      .limit(2);
    const times = (recent ?? []).map((r) => new Date(r.finished_at));
    const windowMs = staleAfterMs(parsePgInterval(repo.stale_after), times);
    const lastSuccess = times[0] ?? null;
    const key = { userId: repo.user_id, kind: "stale_repo" as const, subjectId: repo.id };

    if (isStale(now, lastSuccess, windowMs)) {
      const opened = await openIncident(admin, key, {
        level: "warning",
        title: `Backup tests have gone quiet — ${repo.label}`,
        body:
          `Last successful restore test: ${ago(now.getTime() - lastSuccess!.getTime())}. ` +
          "The watcher may have died — check the agent and its schedule.",
      });
      if (opened) summary.stale_opened++;
    } else {
      const resolved = await resolveIncident(admin, key, {
        level: "recovery",
        title: `Backup tests running again — ${repo.label}`,
        body: "Restore tests are back within their expected window.",
      });
      if (resolved) summary.stale_resolved++;
    }
  }

  // ── silent agents ─────────────────────────────────────────────────────────
  const { data: agents } = await admin
    .from("agents")
    .select("id, user_id, name, last_seen, alert_when_silent")
    .eq("alert_when_silent", true)
    .not("last_seen", "is", null);
  for (const agent of agents ?? []) {
    const silentMs = now.getTime() - new Date(agent.last_seen!).getTime();
    const key = { userId: agent.user_id, kind: "agent_silent" as const, subjectId: agent.id };
    if (silentMs > AGENT_SILENT_MS) {
      const opened = await openIncident(admin, key, {
        level: "warning",
        title: `Agent has gone silent — ${agent.name}`,
        body: `No check-in for ${ago(silentMs)}. The machine may be down.`,
      });
      if (opened) summary.silent_opened++;
    } else {
      const resolved = await resolveIncident(admin, key, {
        level: "recovery",
        title: `Agent is back online — ${agent.name}`,
        body: "The agent is checking in again.",
      });
      if (resolved) summary.silent_resolved++;
    }
  }

  return Response.json(summary);
}
