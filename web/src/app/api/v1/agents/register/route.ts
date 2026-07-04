import { generateApiKey, hashSecret } from "@/lib/api/keys";
import { registerRequestSchema } from "@/lib/api/schemas";
import { createAdminClient } from "@/lib/supabase/admin";

// POST /api/v1/agents/register
// Exchanges a one-time registration token (minted in the dashboard) for an
// agent API key. The key is returned exactly once; only its hash is stored.
export async function POST(request: Request) {
  let body: unknown;
  try {
    body = await request.json();
  } catch {
    return Response.json({ error: "request body must be JSON" }, { status: 400 });
  }
  const parsed = registerRequestSchema.safeParse(body);
  if (!parsed.success) {
    return Response.json(
      { error: "invalid request", details: parsed.error.flatten() },
      { status: 400 },
    );
  }

  const admin = createAdminClient();

  // Consume the token atomically: the WHERE clause only matches unused,
  // unexpired tokens, so a replay loses the race by construction.
  const { data: token, error } = await admin
    .from("registration_tokens")
    .update({ used_at: new Date().toISOString() })
    .eq("token_hash", hashSecret(parsed.data.token))
    .is("used_at", null)
    .gt("expires_at", new Date().toISOString())
    .select("user_id")
    .maybeSingle();
  if (error) {
    return Response.json({ error: "registration failed" }, { status: 500 });
  }
  if (!token) {
    return Response.json(
      { error: "registration token is invalid, expired, or already used" },
      { status: 403 },
    );
  }

  const apiKey = generateApiKey();
  const { data: agent, error: insertError } = await admin
    .from("agents")
    .insert({
      user_id: token.user_id,
      name: parsed.data.name,
      key_hash: hashSecret(apiKey),
      agent_version: parsed.data.agent_version ?? null,
      last_seen: new Date().toISOString(),
    })
    .select("id")
    .single();
  if (insertError || !agent) {
    return Response.json({ error: "registration failed" }, { status: 500 });
  }

  return Response.json({ agent_id: agent.id, api_key: apiKey }, { status: 201 });
}
