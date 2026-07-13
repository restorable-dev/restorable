"use client";

import { track as vercelTrack } from "@vercel/analytics";

// Thin wrapper over Vercel Web Analytics custom events, so call sites stay
// analytics-provider-agnostic. Safe to call anywhere; no-ops if analytics
// isn't loaded (local dev, or before the script initializes).
export function track(event: string, props?: Record<string, string>) {
  vercelTrack(event, props);
}
