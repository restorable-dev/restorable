import { describe, expect, it } from "vitest";

import { formatDuration } from "./format";

describe("formatDuration", () => {
  it.each([
    [0, "0s"],
    [499, "0s"],
    [1000, "1s"],
    [59_000, "59s"],
    [60_000, "1m"],
    [92_000, "1m 32s"],
    [3_600_000, "1h"],
    [5_460_000, "1h 31m"],
  ])("formats %i ms as %s", (ms, expected) => {
    expect(formatDuration(ms)).toBe(expected);
  });

  it("rejects negative durations", () => {
    expect(() => formatDuration(-1)).toThrow(RangeError);
  });

  it("rejects non-finite durations", () => {
    expect(() => formatDuration(Number.NaN)).toThrow(RangeError);
  });
});
