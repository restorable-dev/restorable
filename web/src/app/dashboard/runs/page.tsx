import Link from "next/link";

import { StatusBadge } from "@/components/status-badge";
import { formatDuration } from "@/lib/format";
import { createClient } from "@/lib/supabase/server";
import type { Repo, TestRun } from "@/lib/types";

export const dynamic = "force-dynamic";

export default async function RunsPage({
  searchParams,
}: {
  searchParams: Promise<{ repo?: string }>;
}) {
  const { repo: repoFilter } = await searchParams;
  const supabase = await createClient();

  let query = supabase
    .from("test_runs")
    .select("id, repo_id, status, snapshot_id, started_at, finished_at, created_at, results")
    .order("created_at", { ascending: false })
    .limit(100);
  if (repoFilter) {
    query = query.eq("repo_id", repoFilter);
  }
  const { data: runs } = await query.returns<TestRun[]>();

  const { data: repos } = await supabase
    .from("repos")
    .select("id, fingerprint, label, created_at")
    .returns<Repo[]>();
  const repoById = new Map((repos ?? []).map((r) => [r.id, r]));

  return (
    <main>
      <h1 className="mb-1 text-xl font-semibold">
        Run history
        {repoFilter && repoById.get(repoFilter) ? ` — ${repoById.get(repoFilter)!.label}` : ""}
      </h1>
      <p className="mb-6 text-sm text-neutral-500">Most recent first.</p>

      {!runs?.length ? (
        <p className="text-sm text-neutral-500">No runs recorded yet.</p>
      ) : (
        <table className="w-full text-left text-sm">
          <thead>
            <tr className="border-b border-neutral-200 text-neutral-500 dark:border-neutral-800">
              <th className="py-2 font-normal">Result</th>
              <th className="py-2 font-normal">Repository</th>
              <th className="py-2 font-normal">Snapshot</th>
              <th className="py-2 font-normal">Duration</th>
              <th className="py-2 font-normal">Finished</th>
            </tr>
          </thead>
          <tbody>
            {runs.map((run) => (
              <tr key={run.id} className="border-b border-neutral-100 dark:border-neutral-900">
                <td className="py-3">
                  <Link href={`/dashboard/runs/${run.id}`}>
                    <StatusBadge status={run.status} />
                  </Link>
                </td>
                <td className="py-3">{repoById.get(run.repo_id)?.label ?? "…"}</td>
                <td className="py-3 font-mono text-xs">
                  {run.snapshot_id?.slice(0, 8) ?? "—"}
                </td>
                <td className="py-3 text-neutral-500">
                  {formatDuration(
                    new Date(run.finished_at).getTime() - new Date(run.started_at).getTime(),
                  )}
                </td>
                <td className="py-3 text-neutral-500">
                  <Link href={`/dashboard/runs/${run.id}`} className="hover:underline">
                    {new Date(run.finished_at).toLocaleString()}
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </main>
  );
}
