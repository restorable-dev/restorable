// Row shapes the dashboard reads (subset of the DB schema).

export type RunStatus = "pass" | "fail" | "error";

export interface Repo {
  id: string;
  fingerprint: string;
  label: string;
  created_at: string;
}

export interface CheckResult {
  recipe: string;
  type: string;
  status: RunStatus;
  message: string;
  duration_ms: number;
}

export interface TestRun {
  id: string;
  repo_id: string;
  agent_id: string | null;
  snapshot_id: string | null;
  status: RunStatus;
  error: string | null;
  results: CheckResult[];
  agent_version: string | null;
  started_at: string;
  finished_at: string;
  /** Restore phase only (vs verification); null from pre-v0.1.0 agents. */
  restore_duration_ms: number | null;
  created_at: string;
}

export interface Agent {
  id: string;
  name: string;
  agent_version: string | null;
  last_seen: string | null;
  created_at: string;
}
