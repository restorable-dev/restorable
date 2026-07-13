import Link from "next/link";

import { CopyInstall, SignupCta } from "./landing-cta";

const checkRows = [
  ["files", "Required paths restored, file counts hold, sampled files hash identically to repository content"],
  ["sqlite", "The database passes PRAGMA integrity_check"],
  ["postgres / mysql", "The dump in your backup actually loads into a throwaway container; row counts hold"],
  ["docker-app", "The real app image boots against restored data and answers its health check"],
] as const;

export default function Home() {
  return (
    <main className="mx-auto w-full max-w-3xl px-6">
      {/* Hero */}
      <section className="flex min-h-[70vh] flex-col justify-center gap-6 py-16">
        <p className="font-mono text-sm text-neutral-500">restorable — open-source restore testing for restic</p>
        <h1 className="text-4xl font-semibold leading-tight sm:text-5xl">
          A backup you&apos;ve never restored is a hope, not a backup.
        </h1>
        <p className="max-w-xl text-lg text-neutral-600 dark:text-neutral-400">
          Your backups run every night. Restorable proves they&apos;d actually
          restore: on a schedule, it restores your latest snapshot into a
          disposable sandbox, verifies the data really works, destroys the
          sandbox, and tells you the result.
        </p>
        <CopyInstall command="curl -fsSL https://raw.githubusercontent.com/restorable-dev/restorable/main/install.sh | sh" />
        <div className="flex items-center gap-4">
          <SignupCta />
          <a
            href="https://github.com/restorable-dev/restorable"
            className="text-sm text-neutral-600 underline dark:text-neutral-400"
          >
            Source on GitHub (MIT)
          </a>
        </div>
      </section>

      {/* Proof */}
      <section className="border-t border-neutral-200 py-16 dark:border-neutral-800">
        <h2 className="mb-6 text-2xl font-semibold">Not a green checkmark. A receipt.</h2>
        <pre className="overflow-x-auto rounded-lg bg-neutral-950 p-5 font-mono text-sm leading-relaxed text-neutral-100">
{`$ restorable test
restorable: PASS
  repo:      /srv/backups/restic
  snapshot:  cd10c302
  duration:  5.5s
  checks:
    ✓ nextcloud/files   2 required path(s) present,
                        5 sampled checksum(s) match repository
    ✓ nextcloud/sqlite  "data/owncloud.db" passes integrity_check`}
        </pre>
        <p className="mt-4 max-w-xl text-neutral-600 dark:text-neutral-400">
          Every run is a real restore into a throwaway sandbox — then real
          verification against what your application needs to come back to
          life. Exit codes for cron. JSON for scripts. No account required.
        </p>
      </section>

      {/* Checks */}
      <section className="border-t border-neutral-200 py-16 dark:border-neutral-800">
        <h2 className="mb-6 text-2xl font-semibold">Verification that understands your apps</h2>
        <div className="flex flex-col gap-3">
          {checkRows.map(([name, what]) => (
            <div key={name} className="flex gap-4 rounded border border-neutral-200 p-4 dark:border-neutral-800">
              <code className="shrink-0 font-mono text-sm font-semibold">{name}</code>
              <p className="text-sm text-neutral-600 dark:text-neutral-400">{what}</p>
            </div>
          ))}
        </div>
        <p className="mt-4 text-sm text-neutral-500">
          Ready-made recipes for Nextcloud, Vaultwarden, and Immich ship in the box.
        </p>
      </section>

      {/* Guarantees */}
      <section className="border-t border-neutral-200 py-16 dark:border-neutral-800">
        <h2 className="mb-6 text-2xl font-semibold">Built for people who read the source</h2>
        <ul className="flex flex-col gap-4 text-neutral-600 dark:text-neutral-400">
          <li>
            <strong className="text-neutral-900 dark:text-neutral-100">Read-only, enforced by the compiler.</strong>{" "}
            The agent wraps your restic and can only run restore/ls/check/dump —
            the whitelist is a type, so forget and prune are compile errors.
          </li>
          <li>
            <strong className="text-neutral-900 dark:text-neutral-100">Your data never leaves your machine.</strong>{" "}
            Cloud mode reports pass/fail metadata only: fingerprints, snapshot
            IDs, timings. Never file contents, never credentials. Outbound
            HTTPS only, no open ports.
          </li>
          <li>
            <strong className="text-neutral-900 dark:text-neutral-100">Genuinely useful standalone.</strong>{" "}
            The MIT-licensed agent is the whole core loop. The cloud is
            optional and additive — history, alerting, stale detection.
          </li>
        </ul>
      </section>

      {/* Pricing */}
      <section className="border-t border-neutral-200 py-16 dark:border-neutral-800">
        <h2 className="mb-6 text-2xl font-semibold">Pricing</h2>
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="rounded-lg border border-neutral-200 p-6 dark:border-neutral-800">
            <h3 className="text-lg font-semibold">Free</h3>
            <p className="mb-4 text-sm text-neutral-500">The tool, plus a dashboard</p>
            <ul className="flex flex-col gap-1 text-sm text-neutral-600 dark:text-neutral-400">
              <li>Unlimited standalone use (forever, MIT)</li>
              <li>1 repository on the dashboard</li>
              <li>Monthly scheduled tests</li>
              <li>1 alert channel</li>
            </ul>
          </div>
          <div className="rounded-lg border-2 border-neutral-900 p-6 dark:border-neutral-100">
            <h3 className="text-lg font-semibold">Pro — $8/mo or $80/yr</h3>
            <p className="mb-4 text-sm text-neutral-500">Convenience + evidence</p>
            <ul className="flex flex-col gap-1 text-sm text-neutral-600 dark:text-neutral-400">
              <li>Unlimited repositories</li>
              <li>Any schedule</li>
              <li>All alert channels + stale detection</li>
              <li>12-month run history</li>
              <li>Monthly confidence reports (coming soon)</li>
            </ul>
          </div>
        </div>
        <div className="mt-6">
          <SignupCta />
        </div>
      </section>

      <footer className="flex items-center justify-between border-t border-neutral-200 py-10 text-sm text-neutral-500 dark:border-neutral-800">
        <span>© {new Date().getFullYear()} ClevTech Solutions · MIT licensed</span>
        <div className="flex gap-4">
          <a href="https://github.com/restorable-dev/restorable" className="underline">GitHub</a>
          <Link href="/privacy" className="underline">Privacy</Link>
          <Link href="/terms" className="underline">Terms</Link>
          <Link href="/login" className="underline">Sign in</Link>
        </div>
      </footer>
    </main>
  );
}
