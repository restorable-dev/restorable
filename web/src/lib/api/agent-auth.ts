import "server-only";

import type { SupabaseClient } from "@supabase/supabase-js";

import { API_KEY_PREFIX, hashSecret } from "./keys";

export interface AuthedAgent {
  id: string;
  user_id: string;
}

// authenticateAgent resolves the Authorization: Bearer rsk_… header to an
// agent row, or null. Callers must scope every subsequent query to
// agent.user_id — the admin client bypasses RLS.
export async function authenticateAgent(
  admin: SupabaseClient,
  request: Request,
): Promise<AuthedAgent | null> {
  const header = request.headers.get("authorization") ?? "";
  const [scheme, key] = header.split(" ");
  if (scheme?.toLowerCase() !== "bearer" || !key?.startsWith(API_KEY_PREFIX)) {
    return null;
  }
  const { data, error } = await admin
    .from("agents")
    .select("id, user_id")
    .eq("key_hash", hashSecret(key))
    .maybeSingle();
  if (error || !data) {
    return null;
  }
  return data;
}

export function unauthorized(): Response {
  return Response.json({ error: "invalid or missing agent API key" }, { status: 401 });
}
