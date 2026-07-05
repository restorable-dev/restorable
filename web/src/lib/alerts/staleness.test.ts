import { describe, expect, it } from "vitest";

import { isStale, parsePgInterval, staleAfterMs } from "./staleness";

const HOUR = 3_600_000;
const DAY = 24 * HOUR;

describe("staleAfterMs", () => {
  it("prefers the explicit stale_after", () => {
    expect(staleAfterMs(2 * DAY, [new Date(), new Date(Date.now() - HOUR)])).toBe(2 * DAY);
  });

  it("infers 1.5x cadence from the last two runs", () => {
    const newest = new Date("2026-07-04T00:00:00Z");
    const previous = new Date("2026-07-02T00:00:00Z"); // 2-day cadence
    expect(staleAfterMs(null, [newest, previous])).toBe(3 * DAY);
  });

  it("floors the inferred window at 24h", () => {
    const newest = new Date("2026-07-04T02:00:00Z");
    const previous = new Date("2026-07-04T00:00:00Z"); // 2h cadence
    expect(staleAfterMs(null, [newest, previous])).toBe(DAY);
  });

  it("returns null with fewer than two runs", () => {
    expect(staleAfterMs(null, [new Date()])).toBeNull();
    expect(staleAfterMs(null, [])).toBeNull();
  });
});

describe("isStale", () => {
  const now = new Date("2026-07-04T12:00:00Z");
  it("fires past the window and not before", () => {
    expect(isStale(now, new Date("2026-07-01T12:00:00Z"), 2 * DAY)).toBe(true);
    expect(isStale(now, new Date("2026-07-03T12:00:00Z"), 2 * DAY)).toBe(false);
  });
  it("never fires without a window or history", () => {
    expect(isStale(now, new Date(), null)).toBe(false);
    expect(isStale(now, null, DAY)).toBe(false);
  });
});

describe("parsePgInterval", () => {
  it.each([
    ["48:00:00", 48 * HOUR],
    ["2 days", 2 * DAY],
    ["1 day 12:00:00", DAY + 12 * HOUR],
    ["00:30:00", 30 * 60 * 1000],
  ])("parses %s", (input, want) => {
    expect(parsePgInterval(input)).toBe(want);
  });

  it("returns null for null or unparseable", () => {
    expect(parsePgInterval(null)).toBeNull();
    expect(parsePgInterval("soon")).toBeNull();
  });
});
