import Link from "next/link";

import { createClient } from "@/lib/supabase/server";

import { signOut } from "./actions";

export default async function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();

  return (
    <div className="mx-auto max-w-4xl p-6">
      <header className="mb-8 flex flex-wrap items-center justify-between gap-x-6 gap-y-2 border-b border-neutral-200 pb-4 dark:border-neutral-800">
        <nav className="flex items-center gap-5 text-sm">
          <Link href="/dashboard" className="flex items-center gap-2 font-semibold">
            <svg width="20" height="20" viewBox="0 0 32 32" fill="none" aria-hidden="true">
              <path
                d="M16 4.5 L26 8 V15.5 C26 22 21.6 26.3 16 28 C10.4 26.3 6 22 6 15.5 V8 Z"
                fill="none"
                stroke="#22C55E"
                strokeWidth="2"
                strokeLinejoin="round"
              />
              <path
                d="M11.5 16.2 L14.8 19.5 L20.8 12.5"
                fill="none"
                stroke="#22C55E"
                strokeWidth="2.4"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
            Restorable
          </Link>
          <Link href="/dashboard" className="text-neutral-600 hover:underline dark:text-neutral-400">
            Repos
          </Link>
          <Link
            href="/dashboard/runs"
            className="text-neutral-600 hover:underline dark:text-neutral-400"
          >
            Runs
          </Link>
          <Link
            href="/dashboard/agents"
            className="text-neutral-600 hover:underline dark:text-neutral-400"
          >
            Agents
          </Link>
          <Link
            href="/dashboard/alerts"
            className="text-neutral-600 hover:underline dark:text-neutral-400"
          >
            Alerts
          </Link>
          <Link
            href="/dashboard/billing"
            className="text-neutral-600 hover:underline dark:text-neutral-400"
          >
            Billing
          </Link>
        </nav>
        <form action={signOut} className="flex items-center gap-3 text-sm">
          <span className="text-neutral-500">{user?.email}</span>
          <button type="submit" className="text-neutral-600 underline dark:text-neutral-400">
            Sign out
          </button>
        </form>
      </header>
      {children}
    </div>
  );
}
