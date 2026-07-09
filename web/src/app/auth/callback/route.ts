import { NextResponse } from "next/server";

import { createClient } from "@/lib/supabase/server";

// OAuth / magic-link / recovery callback: exchanges the auth code for a
// session, then continues to `next` (allowlisted paths only) or the dashboard.
const allowedNext = new Set(["/dashboard", "/reset-password"]);

export async function GET(request: Request) {
  const url = new URL(request.url);
  const code = url.searchParams.get("code");
  if (code) {
    const supabase = await createClient();
    await supabase.auth.exchangeCodeForSession(code);
  }
  const next = url.searchParams.get("next") ?? "/dashboard";
  const target = allowedNext.has(next) ? next : "/dashboard";
  return NextResponse.redirect(new URL(target, url.origin));
}
