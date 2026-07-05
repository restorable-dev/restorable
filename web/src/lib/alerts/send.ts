import "server-only";

import { parseChannelConfig, type AlertChannel } from "./config";

export interface AlertMessage {
  title: string;
  body: string;
  level: "failure" | "warning" | "recovery";
}

const emoji: Record<AlertMessage["level"], string> = {
  failure: "❌",
  warning: "⚠️",
  recovery: "✅",
};

const SEND_TIMEOUT_MS = 10_000;

async function post(url: string, init: RequestInit): Promise<Response> {
  const resp = await fetch(url, {
    ...init,
    method: "POST",
    signal: AbortSignal.timeout(SEND_TIMEOUT_MS),
  });
  if (!resp.ok) {
    const body = (await resp.text()).slice(0, 200);
    throw new Error(`HTTP ${resp.status}: ${body}`);
  }
  return resp;
}

// sendToChannel delivers one alert to one channel. Throws with a
// user-presentable message on failure (shown by the channel Test button).
export async function sendToChannel(
  channel: AlertChannel,
  msg: AlertMessage,
): Promise<void> {
  const text = `${emoji[msg.level]} ${msg.title}\n${msg.body}`;
  switch (channel.type) {
    case "telegram": {
      const { chat_id } = parseChannelConfig("telegram", channel.config);
      const token = process.env.TELEGRAM_BOT_TOKEN;
      if (!token) {
        throw new Error("Telegram is not configured on this deployment (TELEGRAM_BOT_TOKEN)");
      }
      const base = process.env.TELEGRAM_API_BASE ?? "https://api.telegram.org";
      await post(`${base}/bot${token}/sendMessage`, {
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ chat_id, text }),
      });
      return;
    }
    case "discord": {
      const { webhook_url } = parseChannelConfig("discord", channel.config);
      await post(webhook_url, {
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ content: text }),
      });
      return;
    }
    case "ntfy": {
      const { topic, server } = parseChannelConfig("ntfy", channel.config);
      const base = (server ?? "https://ntfy.sh").replace(/\/+$/, "");
      await post(`${base}/${topic}`, {
        headers: {
          Title: msg.title,
          Priority: msg.level === "recovery" ? "default" : "high",
          Tags: msg.level === "recovery" ? "white_check_mark" : "warning",
        },
        body: msg.body,
      });
      return;
    }
    case "email": {
      const { to } = parseChannelConfig("email", channel.config);
      const apiKey = process.env.RESEND_API_KEY;
      if (!apiKey) {
        throw new Error("Email is not configured on this deployment (RESEND_API_KEY)");
      }
      await post("https://api.resend.com/emails", {
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${apiKey}`,
        },
        body: JSON.stringify({
          from: process.env.ALERT_EMAIL_FROM ?? "Restorable <alerts@restorable.dev>",
          to: [to],
          subject: `${emoji[msg.level]} ${msg.title}`,
          text: msg.body,
        }),
      });
      return;
    }
  }
}
