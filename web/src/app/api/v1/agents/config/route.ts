import { authenticateAgent, unauthorized } from "@/lib/api/agent-auth";
import { getLimits, getPlan, isBeta } from "@/lib/billing/entitlements";
import { createAdminClient } from "@/lib/supabase/admin";

// GET /api/v1/agents/config
// Polled by agents (~5 min). Returns the desired schedule and the plan's
// entitlements, live from the subscription — an upgrade reflects here on the
// next poll, no redeploy. Server-driven scheduling is post-launch.
export async function GET(request: Request) {
  const admin = createAdminClient();
  const agent = await authenticateAgent(admin, request);
  if (!agent) {
    return unauthorized();
  }
  await admin
    .from("agents")
    .update({ last_seen: new Date().toISOString() })
    .eq("id", agent.id);

  const limits = await getLimits(admin, agent.user_id);
  const plan = isBeta() ? "beta" : await getPlan(admin, agent.user_id);
  return Response.json({
    schedule: null,
    plan,
    entitlements: {
      max_repos: limits.maxRepos,
      min_interval: limits.minInterval,
      max_alert_channels: limits.maxAlertChannels,
    },
  });
}
