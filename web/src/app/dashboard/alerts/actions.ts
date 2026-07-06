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

// Lists chats that recently messaged the bot, so users can pick their
// chat_id instead of hunting for it.
export async function detectTelegramChats(): Promise<{
  chats?: { id: string; name: string }[];
  error?: string;
}> {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) {
    return { error: "not signed in" };
  }
  const token = process.env.TELEGRAM_BOT_TOKEN;
  if (!token) {
    return { error: "Telegram is not configured on this deployment" };
  }
  const base = process.env.TELEGRAM_API_BASE ?? "https://api.telegram.org";
  try {
    const resp = await fetch(`${base}/bot${token}/getUpdates`, {
      signal: AbortSignal.timeout(10_000),
      cache: "no-store",
    });
    if (!resp.ok) {
      return { error: `Telegram API error (HTTP ${resp.status})` };
    }
    const data = (await resp.json()) as {
      result?: { message?: { chat?: { id: number; first_name?: string; title?: string; username?: string } } }[];
    };
    const byId = new Map<string, string>();
    for (const update of data.result ?? []) {
      const chat = update.message?.chat;
      if (chat) {
        byId.set(String(chat.id), chat.title ?? chat.username ?? chat.first_name ?? "chat");
      }
    }
    if (byId.size === 0) {
      return { error: "no messages found — send the bot any message first, then retry" };
    }
    return { chats: [...byId].map(([id, name]) => ({ id, name })) };
  } catch {
    return { error: "could not reach the Telegram API" };
  }
}
