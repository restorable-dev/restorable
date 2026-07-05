import "server-only";

import type { SupabaseClient } from "@supabase/supabase-js";

import type { ChannelType } from "./config";
import { sendToChannel, type AlertMessage } from "./send";

export type AlertKind = "run_failure" | "stale_repo" | "agent_silent";

export interface IncidentKey {
  userId: string;
  kind: AlertKind;
  subjectId: string; // repo id or agent id
}

// dispatchToUserChannels sends msg to every VERIFIED channel of the user.
// Channel failures are isolated: one broken webhook must not stop the rest
// (or the caller). Returns how many channels succeeded.
export async function dispatchToUserChannels(
  admin: SupabaseClient,
  userId: string,
  msg: AlertMessage,
): Promise<number> {
  const { data: channels } = await admin
    .from("alert_channels")
    .select("id, type, config")
    .eq("user_id", userId)
    .eq("verified", true);
  if (!channels?.length) {
    return 0;
  }
  const results = await Promise.allSettled(
    channels.map((c) =>
      sendToChannel({ id: c.id, type: c.type as ChannelType, config: c.config }, msg),
    ),
  );
  for (const r of results) {
    if (r.status === "rejected") {
      console.error("alert channel send failed:", r.reason);
    }
  }
  return results.filter((r) => r.status === "fulfilled").length;
}

// openIncident opens a firing incident and dispatches the alert — unless one
// is already open for this (user, kind, subject), in which case it does
// nothing. The partial unique index makes this race-safe: exactly one insert
// wins, so exactly one alert is sent per incident.
export async function openIncident(
  admin: SupabaseClient,
  key: IncidentKey,
  msg: AlertMessage,
  runId?: string,
): Promise<boolean> {
  const { data, error } = await admin
    .from("alerts")
    .insert({
      user_id: key.userId,
      kind: key.kind,
      subject_id: key.subjectId,
      run_id: runId ?? null,
      message: `${msg.title} — ${msg.body}`,
    })
    .select("id")
    .maybeSingle();
  if (error) {
    // 23505 = unique violation: an incident is already firing. Suppress.
    if (error.code === "23505") {
      return false;
    }
    console.error("open incident failed:", error);
    return false;
  }
  if (!data) {
    return false;
  }
  await dispatchToUserChannels(admin, key.userId, msg);
  return true;
}

// resolveIncident resolves an open incident, if any, and dispatches the
// recovery notice. No open incident → no notice (nothing was alerted).
export async function resolveIncident(
  admin: SupabaseClient,
  key: IncidentKey,
  msg: AlertMessage,
): Promise<boolean> {
  const { data, error } = await admin
    .from("alerts")
    .update({ status: "resolved", resolved_at: new Date().toISOString() })
    .eq("user_id", key.userId)
    .eq("kind", key.kind)
    .eq("subject_id", key.subjectId)
    .eq("status", "firing")
    .select("id");
  if (error) {
    console.error("resolve incident failed:", error);
    return false;
  }
  if (!data?.length) {
    return false;
  }
  await dispatchToUserChannels(admin, key.userId, msg);
  return true;
}
