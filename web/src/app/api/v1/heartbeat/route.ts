import { authenticateAgent, unauthorized } from "@/lib/api/agent-auth";
import { createAdminClient } from "@/lib/supabase/admin";

// POST /api/v1/heartbeat
// Liveness ping. Stale detection (Phase 4) alerts when an agent goes silent.
export async function POST(request: Request) {
  const admin = createAdminClient();
  const agent = await authenticateAgent(admin, request);
  if (!agent) {
    return unauthorized();
  }
  const { error } = await admin
    .from("agents")
    .update({ last_seen: new Date().toISOString() })
    .eq("id", agent.id);
  if (error) {
    return Response.json({ error: "heartbeat failed" }, { status: 500 });
  }
  return new Response(null, { status: 204 });
}
