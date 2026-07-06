"use client";

import Link from "next/link";
import { useState } from "react";

import { track } from "@/lib/analytics";

// Conversion tracking: landing pageviews are automatic; these events measure
// the landing → signup funnel (SPEC: the metric that matters is activation).

export function SignupCta() {
  return (
    <Link
      href="/login"
      onClick={() => track("signup-intent")}
      className="w-fit rounded bg-neutral-900 px-5 py-2.5 text-white dark:bg-white dark:text-neutral-900"
    >
      Get started free
    </Link>
  );
}

export function CopyInstall({ command }: { command: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div className="flex max-w-xl items-center gap-2 rounded-lg border border-neutral-200 bg-neutral-50 p-3 dark:border-neutral-800 dark:bg-neutral-900">
      <code className="flex-1 overflow-x-auto whitespace-nowrap font-mono text-xs">
        {command}
      </code>
      <button
        onClick={() => {
          void navigator.clipboard.writeText(command);
          setCopied(true);
          track("copy-install");
          setTimeout(() => setCopied(false), 1500);
        }}
        className="shrink-0 rounded border border-neutral-300 px-2 py-1 text-xs dark:border-neutral-700"
      >
        {copied ? "copied" : "copy"}
      </button>
    </div>
  );
}
