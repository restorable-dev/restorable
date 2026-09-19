import { describe, expect, it } from "vitest";

import { registerRequestSchema, runRequestSchema } from "./schemas";

const validRun = {
  repo_fingerprint: "a".repeat(64),
  repo_label: "/srv/backups/restic",
  snapshot_id: "abc123",
  status: "pass",
  started_at: "2026-07-04T03:00:00Z",
  finished_at: "2026-07-04T03:01:30.123456-04:00",
  agent_version: "v0.1.0",
  checks: [
    { recipe: "nextcloud", type: "files", status: "pass", message: "ok", duration_ms: 42 },
  ],
};

describe("runRequestSchema", () => {
  it("accepts a valid payload (Z and offset timestamps)", () => {
    expect(runRequestSchema.safeParse(validRun).success).toBe(true);
  });

  it("defaults missing check message to empty string", () => {
    const parsed = runRequestSchema.parse({
      ...validRun,
      checks: [{ recipe: "r", type: "files", status: "fail", duration_ms: 1 }],
    });
    expect(parsed.checks[0].message).toBe("");
  });

  it("accepts restore_duration_ms and leaves it optional", () => {
    expect(
      runRequestSchema.safeParse({ ...validRun, restore_duration_ms: 42500 }).success,
    ).toBe(true);
    const parsed = runRequestSchema.parse(validRun);
    expect(parsed.restore_duration_ms).toBeUndefined();
  });

  it.each([
    ["bad fingerprint", { repo_fingerprint: "not-hex" }],
    ["uppercase fingerprint", { repo_fingerprint: "A".repeat(64) }],
    ["unknown status", { status: "flaky" }],
    ["missing label", { repo_label: "" }],
    ["non-iso timestamp", { started_at: "yesterday" }],
    ["negative duration", { checks: [{ recipe: "r", type: "t", status: "pass", duration_ms: -1 }] }],
    ["negative restore duration", { restore_duration_ms: -1 }],
    ["fractional restore duration", { restore_duration_ms: 1.5 }],
    ["absurd restore duration", { restore_duration_ms: 8 * 24 * 60 * 60 * 1000 }],
  ])("rejects %s", (_name, patch) => {
    expect(runRequestSchema.safeParse({ ...validRun, ...patch }).success).toBe(false);
  });

  it("rejects finished_at before started_at", () => {
    const result = runRequestSchema.safeParse({
      ...validRun,
      started_at: "2026-07-04T05:00:00Z",
      finished_at: "2026-07-04T04:00:00Z",
    });
    expect(result.success).toBe(false);
  });
});

describe("registerRequestSchema", () => {
  it("accepts a valid request", () => {
    expect(
      registerRequestSchema.safeParse({ token: "rrt_abc", name: "homelab-box" }).success,
    ).toBe(true);
  });

  it("rejects tokens without the rrt_ prefix", () => {
    expect(
      registerRequestSchema.safeParse({ token: "rsk_abc", name: "box" }).success,
    ).toBe(false);
  });
});

describe("restore_duration_ms sanity", () => {
  const base = {
    repo_fingerprint: "a".repeat(64),
    repo_label: "/srv/backups/restic",
    status: "pass" as const,
    started_at: "2026-09-19T10:00:00.000Z",
    finished_at: "2026-09-19T10:00:01.000Z", // a one-second run
    agent_version: "v0.1.2",
    checks: [],
  };

  it("rejects a restore longer than the run that contained it", () => {
    // Previously accepted, and the dashboard rendered "1s · restore 10m" as
    // the headline verified recovery time.
    const r = runRequestSchema.safeParse({ ...base, restore_duration_ms: 600_000 });
    expect(r.success).toBe(false);
  });

  it("accepts a restore that fits inside the run", () => {
    expect(runRequestSchema.safeParse({ ...base, restore_duration_ms: 800 }).success).toBe(true);
  });

  it("allows a second of slack for clock granularity", () => {
    expect(runRequestSchema.safeParse({ ...base, restore_duration_ms: 1500 }).success).toBe(true);
  });

  it("still accepts runs that omit it", () => {
    expect(runRequestSchema.safeParse(base).success).toBe(true);
  });
});
