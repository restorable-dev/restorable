import { NextResponse } from "next/server";

import { createClient } from "@/lib/supabase/server";

// OAuth / magic-link / recovery callback: exchanges the auth code for a
// session, then continues to `next` (allowlisted paths only) or the dashboard.
const allowedNext = new Set(["/dashboard", "/reset-password"]);

export async function GET(request: Request) {
  const url = new URL(request.url);

  // GoTrue reports failed verifications (expired or already-used links) as
  // error query params on the redirect. Surface that instead of continuing
  // to a page that can't work without a session.
  if (url.searchParams.get("error") || url.searchParams.get("error_code")) {
    return NextResponse.redirect(new URL("/login?error=link_expired", url.origin));
  }

  const code = url.searchParams.get("code");
  if (code) {
    const supabase = await createClient();
    const { error } = await supabase.auth.exchangeCodeForSession(code);
    if (error) {
      return NextResponse.redirect(new URL("/login?error=link_expired", url.origin));
    }
  }
  const next = url.searchParams.get("next") ?? "/dashboard";
  const target = allowedNext.has(next) ? next : "/dashboard";
  return NextResponse.redirect(new URL(target, url.origin));
}
