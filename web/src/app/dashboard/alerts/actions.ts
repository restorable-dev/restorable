"use server";

import { revalidatePath } from "next/cache";

import { channelTypeSchema, parseChannelConfig, type ChannelType } from "@/lib/alerts/config";
import { sendToChannel } from "@/lib/alerts/send";
import { getLimits } from "@/lib/billing/entitlements";
import { createClient } from "@/lib/supabase/server";

// All actions use the user's own client: RLS scopes every read and write.

export async function createAlertChannel(
  _prev: unknown,
  formData: FormData,
): Promise<{ error?: string }> {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) {
    return { error: "not signed in" };
  }

  const typeParse = channelTypeSchema.safeParse(formData.get("type"));
  if (!typeParse.success) {
    return { error: "unknown channel type" };
  }
  const type = typeParse.data;

  const raw: Record<string, string> = {};
  for (const field of ["chat_id", "webhook_url", "topic", "server", "to"]) {
    const value = formData.get(field);
    if (typeof value === "string" && value.trim() !== "") {
      raw[field] = value.trim();
    }
  }
  let config: unknown;
  try {
    config = parseChannelConfig(type, raw);
  } catch {
    return { error: "invalid channel settings — check the fields and try again" };
  }

  // Plan gate: channel count is a Pro entitlement, enforced server-side.
  const limits = await getLimits(supabase, user.id);
  if (limits.maxAlertChannels != null) {
    const { count } = await supabase
      .from("alert_channels")
      .select("id", { count: "exact", head: true });
    if ((count ?? 0) >= limits.maxAlertChannels) {
      return {
        error: `plan limit: the free plan includes ${limits.maxAlertChannels} alert channel — upgrade to Pro for all channels`,
      };
    }
  }

  const { error } = await supabase.from("alert_channels").insert({
    user_id: user.id,
    type,
    config,
  });
  if (error) {
    return { error: "could not save channel" };
  }
  revalidatePath("/dashboard/alerts");
  return {};
}

export async function deleteAlertChannel(id: string): Promise<void> {
  const supabase = await createClient();
  await supabase.from("alert_channels").delete().eq("id", id);
  revalidatePath("/dashboard/alerts");
}

// Sends a test message through the channel; success marks it verified —
// only verified channels receive real alerts.
export async function testAlertChannel(id: string): Promise<{ error?: string }> {
  const supabase = await createClient();
  const { data: channel } = await supabase
    .from("alert_channels")
    .select("id, type, config")
    .eq("id", id)
    .maybeSingle();
  if (!channel) {
    return { error: "channel not found" };
  }
  try {
    await sendToChannel(
      { id: channel.id, type: channel.type as ChannelType, config: channel.config },
      {
        level: "recovery",
        title: "Test alert from Restorable",
        body: "If you can read this, the channel works. Real alerts will look like this.",
      },
    );
  } catch (err) {
    return { error: err instanceof Error ? err.message : "send failed" };
  }
  await supabase.from("alert_channels").update({ verified: true }).eq("id", id);
  revalidatePath("/dashboard/alerts");
  return {};
}

// Telegram connect via one-time deep link. Mints a token tied to THIS user,
// returns a t.me/<bot>?start=<token> link. The user taps it, the bot webhook
// stamps their chat_id onto this row — privately, per user. Replaces the old
// getUpdates lookup, which surfaced every user's chat across the shared bot.
export async function startTelegramConnect(): Promise<{
  url?: string;
  token?: string;
  error?: string;
}> {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) {
    return { error: "not signed in" };
  }
  const botUser = process.env.TELEGRAM_BOT_USERNAME;
  if (!botUser) {
    return { error: "Telegram is not configured on this deployment" };
  }
  const token = generateConnectToken();
  const { error } = await supabase.from("telegram_links").insert({
    token,
    user_id: user.id,
  });
  if (error) {
    return { error: "could not start Telegram connect" };
  }
  return { url: `https://t.me/${botUser}?start=${token}`, token };
}

// Polls a pending connect: once the webhook has stamped a chat_id, creates the
// verified Telegram channel and consumes the link. Called by the UI on a timer
// after the user opens the deep link.
export async function pollTelegramConnect(
  token: string,
): Promise<{ connected?: boolean; error?: string }> {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) {
    return { error: "not signed in" };
  }
  const { data: link } = await supabase
    .from("telegram_links")
    .select("chat_id, consumed_at")
    .eq("token", token)
    .maybeSingle();
  if (!link || !link.chat_id) {
    return { connected: false };
  }
  if (link.consumed_at) {
    return { connected: true };
  }

  const limits = await getLimits(supabase, user.id);
  if (limits.maxAlertChannels != null) {
    const { count } = await supabase
      .from("alert_channels")
      .select("id", { count: "exact", head: true });
    if ((count ?? 0) >= limits.maxAlertChannels) {
      return {
        error: `plan limit: the free plan includes ${limits.maxAlertChannels} alert channel — upgrade to Pro`,
      };
    }
  }

  const { error: insertErr } = await supabase.from("alert_channels").insert({
    user_id: user.id,
    type: "telegram",
    config: { chat_id: link.chat_id },
    verified: true, // connecting via the bot proves the chat is reachable
  });
  if (insertErr) {
    return { error: "could not save the Telegram channel" };
  }
  await supabase
    .from("telegram_links")
    .update({ consumed_at: new Date().toISOString() })
    .eq("token", token);
  revalidatePath("/dashboard/alerts");
  return { connected: true };
}

function generateConnectToken(): string {
  const bytes = new Uint8Array(24);
  crypto.getRandomValues(bytes);
  return Buffer.from(bytes).toString("base64url");
}
