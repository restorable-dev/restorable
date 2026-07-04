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

  it.each([
    ["bad fingerprint", { repo_fingerprint: "not-hex" }],
    ["uppercase fingerprint", { repo_fingerprint: "A".repeat(64) }],
    ["unknown status", { status: "flaky" }],
    ["missing label", { repo_label: "" }],
    ["non-iso timestamp", { started_at: "yesterday" }],
    ["negative duration", { checks: [{ recipe: "r", type: "t", status: "pass", duration_ms: -1 }] }],
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
