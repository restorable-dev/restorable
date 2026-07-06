import "server-only";

import { SendEmailCommand, SESv2Client } from "@aws-sdk/client-sesv2";

import { parseChannelConfig, type AlertChannel } from "./config";

// Email goes out through AWS SES. Custom env names (SES_*) because Vercel
// reserves the standard AWS_* variables for its own runtime.
async function sendEmailViaSES(to: string, subject: string, body: string): Promise<void> {
  const accessKeyId = process.env.SES_ACCESS_KEY_ID;
  const secretAccessKey = process.env.SES_SECRET_ACCESS_KEY;
  if (!accessKeyId || !secretAccessKey) {
    throw new Error("Email is not configured on this deployment (SES_ACCESS_KEY_ID)");
  }
  const client = new SESv2Client({
    region: process.env.SES_REGION ?? "us-east-1",
    credentials: { accessKeyId, secretAccessKey },
  });
  await client.send(
    new SendEmailCommand({
      FromEmailAddress: process.env.ALERT_EMAIL_FROM ?? "Restorable <alerts@restorable.dev>",
      Destination: { ToAddresses: [to] },
      Content: {
        Simple: {
          Subject: { Data: subject, Charset: "UTF-8" },
          Body: { Text: { Data: body, Charset: "UTF-8" } },
        },
      },
    }),
  );
}

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
      await sendEmailViaSES(to, `${emoji[msg.level]} ${msg.title}`, msg.body);
      return;
    }
  }
}
