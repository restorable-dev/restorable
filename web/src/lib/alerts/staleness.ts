// Staleness policy (SPEC: "last successful run exceeds its schedule window
// × 1.5"). The control plane does not yet store per-repo schedules, so the
// window is the repo's explicit stale_after when set, otherwise inferred
// from the observed cadence: 1.5× the gap between the two most recent runs,
// with a 24h floor. Fewer than two runs → no staleness (nothing to infer).

export const MIN_STALE_AFTER_MS = 24 * 60 * 60 * 1000;
const CADENCE_FACTOR = 1.5;

// staleAfterMs decides the allowed silence window for a repo.
// explicitMs: parsed repos.stale_after, if set.
// recentRunTimes: finished_at of the most recent runs, newest first.
export function staleAfterMs(
  explicitMs: number | null,
  recentRunTimes: Date[],
): number | null {
  if (explicitMs != null && explicitMs > 0) {
    return explicitMs;
  }
  if (recentRunTimes.length < 2) {
    return null;
  }
  const gap = recentRunTimes[0].getTime() - recentRunTimes[1].getTime();
  return Math.max(gap * CADENCE_FACTOR, MIN_STALE_AFTER_MS);
}

// isStale: has the newest successful run drifted past the window?
export function isStale(
  now: Date,
  lastSuccess: Date | null,
  windowMs: number | null,
): boolean {
  if (windowMs == null || lastSuccess == null) {
    return false;
  }
  return now.getTime() - lastSuccess.getTime() > windowMs;
}

// parsePgInterval converts the common postgres interval text forms we store
// ("48:00:00", "2 days", "1 day 12:00:00") to milliseconds.
export function parsePgInterval(interval: string | null): number | null {
  if (!interval) {
    return null;
  }
  let ms = 0;
  let rest = interval.trim();

  const dayMatch = rest.match(/^(\d+)\s+days?\s*/);
  if (dayMatch) {
    ms += Number(dayMatch[1]) * 24 * 60 * 60 * 1000;
    rest = rest.slice(dayMatch[0].length);
  }
  if (rest) {
    const hms = rest.match(/^(\d+):(\d+):(\d+)$/);
    if (!hms) {
      return ms > 0 ? ms : null;
    }
    ms += (Number(hms[1]) * 3600 + Number(hms[2]) * 60 + Number(hms[3])) * 1000;
  }
  return ms > 0 ? ms : null;
}
