"use client";

import { useState } from "react";

import { mintRegistrationToken } from "../actions";

// Mints a registration token and shows it exactly once, with the command to
// run. Refreshing the page loses it — that is the point.
export function NewAgentToken() {
  const [token, setToken] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function mint() {
    setBusy(true);
    setError(null);
    const result = await mintRegistrationToken();
    setBusy(false);
    if (result.error || !result.token) {
      setError(result.error ?? "something went wrong");
      return;
    }
    setToken(result.token);
  }

  if (token) {
    const command = `restorable register --url ${window.location.origin} --token ${token}`;
    return (
      <div className="rounded border border-neutral-300 p-4 text-sm dark:border-neutral-700">
        <p className="mb-2 font-medium">
          Run this on the machine that can reach your backup repository:
        </p>
        <pre className="overflow-x-auto rounded bg-neutral-100 p-3 font-mono text-xs dark:bg-neutral-900">
          {command}
        </pre>
        <p className="mt-2 text-neutral-500">
          This token works once and expires in an hour. It will not be shown again.
        </p>
        <button
          onClick={() => navigator.clipboard.writeText(command)}
          className="mt-2 rounded border border-neutral-300 px-2 py-1 text-xs dark:border-neutral-700"
        >
          Copy command
        </button>
      </div>
    );
  }

  return (
    <div>
      <button
        onClick={mint}
        disabled={busy}
        className="rounded bg-neutral-900 px-3 py-2 text-sm text-white disabled:opacity-50 dark:bg-white dark:text-neutral-900"
      >
        {busy ? "…" : "Register a new agent"}
      </button>
      {error && <p className="mt-2 text-sm text-red-600">{error}</p>}
    </div>
  );
}
