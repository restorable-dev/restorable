import Link from "next/link";

// Placeholder landing page — the real marketing site is Phase 6.
export default function Home() {
  return (
    <main className="mx-auto flex min-h-screen max-w-2xl flex-col items-start justify-center gap-6 p-6">
      <h1 className="text-3xl font-semibold leading-tight">
        A backup you&apos;ve never restored is a hope, not a backup.
      </h1>
      <p className="text-neutral-600 dark:text-neutral-400">
        Restorable restore-tests your backups on a schedule: restore the latest
        snapshot into a disposable sandbox, verify the data actually works,
        destroy the sandbox, report the result.
      </p>
      <Link
        href="/login"
        className="rounded bg-neutral-900 px-4 py-2 text-white dark:bg-white dark:text-neutral-900"
      >
        Sign in
      </Link>
    </main>
  );
}
