import { createServerClient } from "@supabase/ssr";
import { cookies } from "next/headers";

// Server-side Supabase client bound to the request's auth cookies.
// RLS applies: this client only sees the signed-in user's rows.
export async function createClient() {
  const cookieStore = await cookies();
  return createServerClient(
    process.env.NEXT_PUBLIC_SUPABASE_URL!,
    process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY!,
    {
      cookies: {
        getAll() {
          return cookieStore.getAll();
        },
        setAll(cookiesToSet) {
          try {
            for (const { name, value, options } of cookiesToSet) {
              cookieStore.set(name, value, options);
            }
          } catch {
            // Called from a Server Component: session refresh is handled by
            // the proxy, so writes here can be safely ignored.
          }
        },
      },
    },
  );
}
