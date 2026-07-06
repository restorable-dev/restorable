"use client";

import { useState } from "react";

export function BillingActions({
  plan,
  hasCustomer,
}: {
  plan: "free" | "pro";
  hasCustomer: boolean;
}) {
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function go(path: string, body?: object) {
    setBusy(true);
    setError(null);
    try {
      const resp = await fetch(path, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: body ? JSON.stringify(body) : undefined,
      });
      const data = (await resp.json()) as { url?: string; error?: string };
      if (!resp.ok || !data.url) {
        setError(data.error ?? "something went wrong");
        return;
      }
      window.location.assign(data.url);
    } catch {
      setError("network error");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col gap-3">
      {plan === "free" && (
        <div className="flex gap-3">
          <button
            onClick={() => go("/api/billing/checkout", { interval: "month" })}
            disabled={busy}
            className="rounded bg-neutral-900 px-4 py-2 text-sm text-white disabled:opacity-50 dark:bg-white dark:text-neutral-900"
          >
            Upgrade — $8/month
          </button>
          <button
            onClick={() => go("/api/billing/checkout", { interval: "year" })}
            disabled={busy}
            className="rounded bg-neutral-900 px-4 py-2 text-sm text-white disabled:opacity-50 dark:bg-white dark:text-neutral-900"
          >
            Upgrade — $80/year
          </button>
        </div>
      )}
      {hasCustomer && (
        <button
          onClick={() => go("/api/billing/portal")}
          disabled={busy}
          className="w-fit rounded border border-neutral-300 px-4 py-2 text-sm disabled:opacity-50 dark:border-neutral-700"
        >
          Manage subscription
        </button>
      )}
      {error && <p className="text-sm text-red-600">{error}</p>}
    </div>
  );
}
