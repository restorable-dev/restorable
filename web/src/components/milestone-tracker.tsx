"use client";

import { useEffect } from "react";

import { track } from "@/lib/analytics";

// Fires activation-funnel events for milestones that actually happen
// server-side (an agent registering, reporting its first run) but which
// Vercel's browser-only custom events can't see directly. We detect them when
// the dashboard first observes the state, and dedupe per user with
// localStorage so each milestone counts once.
export function MilestoneTracker({
  userId,
  hasAgent,
  hasRun,
}: {
  userId: string;
  hasAgent: boolean;
  hasRun: boolean;
}) {
  useEffect(() => {
    fireOnce(userId, "agent-connected", hasAgent);
    fireOnce(userId, "first-green-test", hasRun);
  }, [userId, hasAgent, hasRun]);

  return null;
}

function fireOnce(userId: string, event: string, condition: boolean) {
  if (!condition) return;
  const key = `rstr:${event}:${userId}`;
  try {
    if (localStorage.getItem(key)) return;
    localStorage.setItem(key, "1");
  } catch {
    // localStorage unavailable (private mode): just fire; minor double-count.
  }
  track(event);
}
