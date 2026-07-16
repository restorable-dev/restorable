import { z } from "zod";

// Zod schemas for every /api/v1 boundary. The agent is trusted code we ship,
// but the endpoint is on the open internet — validate everything.

export const registerRequestSchema = z.object({
  token: z
    .string()
    .startsWith("rrt_")
    .max(100),
  name: z.string().min(1).max(100),
  agent_version: z.string().max(50).optional(),
});

export const checkResultSchema = z.object({
  recipe: z.string().min(1).max(200),
  type: z.string().min(1).max(50),
  status: z.enum(["pass", "fail", "error"]),
  message: z.string().max(4000).default(""),
  duration_ms: z.number().int().nonnegative(),
});

export const runRequestSchema = z
  .object({
    // sha256 hex of the credential-scrubbed repo string, computed agent-side.
    repo_fingerprint: z.string().regex(/^[a-f0-9]{64}$/),
    // Credential-scrubbed, human-readable repo label.
    repo_label: z.string().min(1).max(200),
    snapshot_id: z.string().max(100).optional(),
    status: z.enum(["pass", "fail", "error"]),
    error: z.string().max(4000).optional(),
    started_at: z.iso.datetime({ offset: true }),
    finished_at: z.iso.datetime({ offset: true }),
    // How long the restic restore itself took (vs verification). Optional:
    // absent when the run failed before the restore finished, and from
    // pre-release agents. Capped at 7 days — larger values are clock bugs.
    restore_duration_ms: z
      .number()
      .int()
      .min(0)
      .max(7 * 24 * 60 * 60 * 1000)
      .optional(),
    agent_version: z.string().max(50),
    checks: z.array(checkResultSchema).max(200),
  })
  .refine(
    (r) => new Date(r.finished_at).getTime() >= new Date(r.started_at).getTime(),
    { message: "finished_at must not be before started_at" },
  );

export type RunRequest = z.infer<typeof runRequestSchema>;
export type RegisterRequest = z.infer<typeof registerRequestSchema>;
