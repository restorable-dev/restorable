import { authenticateAgent, unauthorized } from "@/lib/api/agent-auth";
import { resolveIncident } from "@/lib/alerts/incidents";
import { createAdminClient } from "@/lib/supabase/admin";

// POST /api/v1/heartbeat
// Liveness ping. A heartbeat proves the agent is alive, so it also resolves
// any open agent-silent incident with a recovery notice.
export async function POST(request: Request) {
  const admin = createAdminClient();
  const agent = await authenticateAgent(admin, request);
  if (!agent) {
    return unauthorized();
  }
  const { data: updated, error } = await admin
    .from("agents")
    .update({ last_seen: new Date().toISOString() })
    .eq("id", agent.id)
    .select("name")
    .single();
  if (error) {
    return Response.json({ error: "heartbeat failed" }, { status: 500 });
  }
  await resolveIncident(
    admin,
    { userId: agent.user_id, kind: "agent_silent", subjectId: agent.id },
    {
      level: "recovery",
      title: `Agent is back online — ${updated?.name ?? "agent"}`,
      body: "The agent is checking in again.",
    },
  );
  return new Response(null, { status: 204 });
}
