import Link from "next/link";

import { NoRunsBadge, StatusBadge } from "@/components/status-badge";
import { createClient } from "@/lib/supabase/server";
import type { Repo, TestRun } from "@/lib/types";

export const dynamic = "force-dynamic";

export default async function ReposPage() {
  const supabase = await createClient();
  const { data: repos } = await supabase
    .from("repos")
    .select("id, fingerprint, label, created_at")
    .order("created_at", { ascending: true })
    .returns<Repo[]>();

  const repoIds = (repos ?? []).map((r) => r.id);
  const { data: runs } = repoIds.length
    ? await supabase
        .from("test_runs")
        .select("id, repo_id, status, finished_at, created_at")
        .in("repo_id", repoIds)
        .order("created_at", { ascending: false })
        .limit(200)
        .returns<Pick<TestRun, "id" | "repo_id" | "status" | "finished_at" | "created_at">[]>()
    : { data: [] };

  const lastRunByRepo = new Map<string, NonNullable<typeof runs>[number]>();
  for (const run of runs ?? []) {
    if (!lastRunByRepo.has(run.repo_id)) {
      lastRunByRepo.set(run.repo_id, run);
    }
  }

  return (
    <main>
      <h1 className="mb-1 text-xl font-semibold">Repositories</h1>
      <p className="mb-6 text-sm text-neutral-500">
        Every repository your agents have restore-tested.
      </p>

      {!repos?.length ? (
        <div className="rounded border border-dashed border-neutral-300 p-8 text-center text-sm text-neutral-500 dark:border-neutral-700">
          <p className="mb-2">No repositories yet.</p>
          <p>
            <Link href="/dashboard/agents" className="underline">
              Register an agent
            </Link>{" "}
            and run <code>restorable test</code> — the repo appears here with its first result.
          </p>
        </div>
      ) : (
        <table className="w-full text-left text-sm">
          <thead>
            <tr className="border-b border-neutral-200 text-neutral-500 dark:border-neutral-800">
              <th className="py-2 font-normal">Repository</th>
              <th className="py-2 font-normal">Last test</th>
              <th className="py-2 font-normal">When</th>
            </tr>
          </thead>
          <tbody>
            {repos.map((repo) => {
              const last = lastRunByRepo.get(repo.id);
              return (
                <tr key={repo.id} className="border-b border-neutral-100 dark:border-neutral-900">
                  <td className="py-3">
                    <Link href={`/dashboard/runs?repo=${repo.id}`} className="hover:underline">
                      {repo.label}
                    </Link>
                    <div className="text-xs text-neutral-400">
                      {repo.fingerprint.slice(0, 12)}
                    </div>
                  </td>
                  <td className="py-3">
                    {last ? (
                      <Link href={`/dashboard/runs/${last.id}`}>
                        <StatusBadge status={last.status} />
                      </Link>
                    ) : (
                      <NoRunsBadge />
                    )}
                  </td>
                  <td className="py-3 text-neutral-500">
                    {last ? new Date(last.finished_at).toLocaleString() : "—"}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
    </main>
  );
}
