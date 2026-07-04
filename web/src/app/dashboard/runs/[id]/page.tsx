import { notFound } from "next/navigation";

import { StatusBadge } from "@/components/status-badge";
import { formatDuration } from "@/lib/format";
import { createClient } from "@/lib/supabase/server";
import type { Repo, TestRun } from "@/lib/types";

export const dynamic = "force-dynamic";

export default async function RunDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const supabase = await createClient();

  const { data: run } = await supabase
    .from("test_runs")
    .select("*")
    .eq("id", id)
    .maybeSingle<TestRun>();
  if (!run) {
    notFound();
  }

  const { data: repo } = await supabase
    .from("repos")
    .select("id, fingerprint, label, created_at")
    .eq("id", run.repo_id)
    .maybeSingle<Repo>();

  return (
    <main>
      <div className="mb-6 flex items-center gap-3">
        <StatusBadge status={run.status} />
        <h1 className="text-xl font-semibold">{repo?.label ?? "Run detail"}</h1>
      </div>

      <dl className="mb-8 grid grid-cols-2 gap-x-8 gap-y-2 text-sm sm:grid-cols-4">
        <div>
          <dt className="text-neutral-500">Snapshot</dt>
          <dd className="font-mono text-xs">{run.snapshot_id?.slice(0, 12) ?? "—"}</dd>
        </div>
        <div>
          <dt className="text-neutral-500">Duration</dt>
          <dd>
            {formatDuration(
              new Date(run.finished_at).getTime() - new Date(run.started_at).getTime(),
            )}
          </dd>
        </div>
        <div>
          <dt className="text-neutral-500">Finished</dt>
          <dd>{new Date(run.finished_at).toLocaleString()}</dd>
        </div>
        <div>
          <dt className="text-neutral-500">Agent version</dt>
          <dd>{run.agent_version ?? "—"}</dd>
        </div>
      </dl>

      {run.error && (
        <div className="mb-8 rounded border border-amber-300 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-200">
          {run.error}
        </div>
      )}

      <h2 className="mb-3 font-medium">Checks</h2>
      {run.results.length === 0 ? (
        <p className="text-sm text-neutral-500">
          No recipe checks ran (restore-only verification).
        </p>
      ) : (
        <ul className="flex flex-col gap-2">
          {run.results.map((check, i) => (
            <li
              key={i}
              className="rounded border border-neutral-200 p-3 text-sm dark:border-neutral-800"
            >
              <div className="flex items-center gap-3">
                <StatusBadge status={check.status} />
                <span className="font-medium">
                  {check.recipe}/{check.type}
                </span>
                <span className="ml-auto text-xs text-neutral-500">
                  {formatDuration(check.duration_ms)}
                </span>
              </div>
              {check.message && (
                <p className="mt-2 text-neutral-600 dark:text-neutral-400">{check.message}</p>
              )}
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}
