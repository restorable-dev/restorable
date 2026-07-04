import { authenticateAgent, unauthorized } from "@/lib/api/agent-auth";
import { createAdminClient } from "@/lib/supabase/admin";

// GET /api/v1/agents/config
// Polled by agents (~5 min). Returns the desired schedule and plan
// entitlements. Phase 3 ships the shape; server-driven scheduling and plan
// gating land in Phases 4-5.
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

  return Response.json({
    schedule: null,
    plan: "free",
    entitlements: { max_repos: 1, min_interval: "monthly", max_alert_channels: 1 },
  });
}
