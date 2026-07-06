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
      <header className="mb-8 flex items-center justify-between border-b border-neutral-200 pb-4 dark:border-neutral-800">
        <nav className="flex items-center gap-5 text-sm">
          <Link href="/dashboard" className="font-semibold">
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
