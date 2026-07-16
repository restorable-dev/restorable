import Link from "next/link";

import { NoRunsBadge, StatusBadge } from "@/components/status-badge";
import { formatDuration } from "@/lib/format";
import { createClient } from "@/lib/supabase/server";
import type { Repo, TestRun } from "@/lib/types";

export const dynamic = "force-dynamic";

type RunRow = Pick<
  TestRun,
  "id" | "repo_id" | "status" | "started_at" | "finished_at" | "restore_duration_ms" | "created_at"
>;

const runDurationMs = (run: RunRow) =>
  new Date(run.finished_at).getTime() - new Date(run.started_at).getTime();

// Recovery drift: last verified recovery time vs the median of up to ten
// earlier passing runs. Only reported once there's enough history (3+) and
// the change is big enough (±15%) to be signal rather than noise.
function recoveryDrift(passing: RunRow[]): number | null {
  if (passing.length < 4) {
    return null;
  }
  const prior = passing
    .slice(1, 11)
    .map(runDurationMs)
    .sort((a, b) => a - b);
  const median = prior[Math.floor(prior.length / 2)];
  if (median <= 0) {
    return null;
  }
  const delta = (runDurationMs(passing[0]) - median) / median;
  return Math.abs(delta) >= 0.15 ? delta : null;
}

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
        .select("id, repo_id, status, started_at, finished_at, restore_duration_ms, created_at")
        .in("repo_id", repoIds)
        .order("created_at", { ascending: false })
        .limit(200)
        .returns<RunRow[]>()
    : { data: [] };

  const lastRunByRepo = new Map<string, RunRow>();
  const passingByRepo = new Map<string, RunRow[]>();
  for (const run of runs ?? []) {
    if (!lastRunByRepo.has(run.repo_id)) {
      lastRunByRepo.set(run.repo_id, run);
    }
    if (run.status === "pass") {
      const list = passingByRepo.get(run.repo_id) ?? [];
      list.push(run);
      passingByRepo.set(run.repo_id, list);
    }
  }

  return (
    <main>
      <h1 className="mb-1 text-xl font-semibold">Repositories</h1>
      <p className="mb-6 text-sm text-neutral-500">
        Every repository your agents have restore-tested. Recovery time is
        measured from the last passing test — a real restore, not an estimate.
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
              <th className="py-2 font-normal">Verified recovery time</th>
              <th className="py-2 font-normal">When</th>
            </tr>
          </thead>
          <tbody>
            {repos.map((repo) => {
              const last = lastRunByRepo.get(repo.id);
              const passing = passingByRepo.get(repo.id) ?? [];
              const verified = passing[0];
              const drift = recoveryDrift(passing);
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
                  <td className="py-3">
                    {verified ? (
                      <>
                        <span className="font-medium">
                          {formatDuration(runDurationMs(verified))}
                        </span>
                        {drift != null && (
                          <span
                            className={
                              drift > 0
                                ? "ml-2 text-xs text-amber-600 dark:text-amber-400"
                                : "ml-2 text-xs text-green-700 dark:text-green-400"
                            }
                          >
                            {drift > 0 ? "▲" : "▼"} {Math.round(Math.abs(drift) * 100)}% vs
                            typical
                          </span>
                        )}
                        {verified.restore_duration_ms != null && (
                          <div className="text-xs text-neutral-400">
                            restore {formatDuration(verified.restore_duration_ms)} · verify{" "}
                            {formatDuration(
                              Math.max(0, runDurationMs(verified) - verified.restore_duration_ms),
                            )}
                          </div>
                        )}
                      </>
                    ) : (
                      <span className="text-neutral-400">—</span>
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
