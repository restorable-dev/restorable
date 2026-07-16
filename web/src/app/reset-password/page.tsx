"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { createClient } from "@/lib/supabase/client";

// Landed on from the password-recovery email link (the auth callback has
// already established a session by the time the user is here). If no session
// exists — expired link, prefetched by a mail scanner, or a repeated reset
// request superseding this one — say so instead of offering a doomed form.
export default function ResetPasswordPage() {
  const router = useRouter();
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [session, setSession] = useState<"checking" | "ok" | "missing">("checking");

  useEffect(() => {
    const supabase = createClient();
    supabase.auth.getUser().then(({ data }) => {
      setSession(data.user ? "ok" : "missing");
    });
  }, []);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    const supabase = createClient();
    const { error } = await supabase.auth.updateUser({ password });
    setBusy(false);
    if (error) {
      setError(error.message);
      return;
    }
    router.push("/dashboard");
    router.refresh();
  }

  if (session === "checking") {
    return (
      <main className="mx-auto flex min-h-screen max-w-sm flex-col justify-center p-6">
        <p className="text-sm text-neutral-500">Checking your recovery link…</p>
      </main>
    );
  }

  if (session === "missing") {
    return (
      <main className="mx-auto flex min-h-screen max-w-sm flex-col justify-center gap-4 p-6">
        <h1 className="text-2xl font-semibold">This link has expired</h1>
        <p className="text-sm text-neutral-600 dark:text-neutral-400">
          The recovery link is invalid, was already used, or was replaced by a
          newer one (each &ldquo;reset password&rdquo; email cancels the ones
          before it). Request a fresh link and use the newest email.
        </p>
        <Link
          href="/login"
          className="rounded bg-neutral-900 px-3 py-2 text-center text-sm text-white dark:bg-white dark:text-neutral-900"
        >
          Back to sign in
        </Link>
      </main>
    );
  }

  return (
    <main className="mx-auto flex min-h-screen max-w-sm flex-col justify-center gap-6 p-6">
      <div>
        <h1 className="text-2xl font-semibold">Set a new password</h1>
        <p className="text-sm text-neutral-500">You&apos;re signed in via the recovery link.</p>
      </div>
      <form onSubmit={submit} className="flex flex-col gap-3">
        <label className="flex flex-col gap-1 text-sm">
          New password
          <input
            type="password"
            required
            minLength={8}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="rounded border border-neutral-300 px-3 py-2 dark:border-neutral-700 dark:bg-neutral-900"
          />
        </label>
        {error && <p className="text-sm text-red-600">{error}</p>}
        <button
          type="submit"
          disabled={busy}
          className="rounded bg-neutral-900 px-3 py-2 text-white disabled:opacity-50 dark:bg-white dark:text-neutral-900"
        >
          {busy ? "…" : "Save new password"}
        </button>
      </form>
    </main>
  );
}
