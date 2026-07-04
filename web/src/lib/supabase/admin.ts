import "server-only";

import { createClient } from "@supabase/supabase-js";

// Service-role client for /api/v1 agent endpoints. Bypasses RLS — every
// query MUST scope by the authenticated agent's user_id explicitly.
// Never import this from client components.
export function createAdminClient() {
  return createClient(
    process.env.NEXT_PUBLIC_SUPABASE_URL!,
    process.env.SUPABASE_SERVICE_ROLE_KEY!,
    { auth: { persistSession: false, autoRefreshToken: false } },
  );
}
