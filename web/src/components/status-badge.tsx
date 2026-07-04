import type { RunStatus } from "@/lib/types";

const styles: Record<RunStatus, string> = {
  pass: "bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300",
  fail: "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300",
  error: "bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300",
};

export function StatusBadge({ status }: { status: RunStatus }) {
  return (
    <span
      className={`inline-block rounded px-2 py-0.5 text-xs font-medium uppercase ${styles[status]}`}
    >
      {status}
    </span>
  );
}

export function NoRunsBadge() {
  return (
    <span className="inline-block rounded bg-neutral-100 px-2 py-0.5 text-xs font-medium uppercase text-neutral-500 dark:bg-neutral-800 dark:text-neutral-400">
      no runs
    </span>
  );
}
