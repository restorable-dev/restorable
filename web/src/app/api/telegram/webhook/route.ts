import { createAdminClient } from "@/lib/supabase/admin";

// POST /api/telegram/webhook
//
// Telegram calls this when someone messages the bot. We only care about
// `/start <token>`: the token identifies which Restorable user initiated the
// connect, so we can privately stamp their chat_id onto their own pending
// link — no getUpdates, no cross-user exposure. Authenticated by a secret
// token Telegram echoes back in a header (set at setWebhook time).

interface TelegramUpdate {
  message?: {
    text?: string;
    chat?: { id: number; first_name?: string; username?: string; title?: string };
  };
}

async function replyToChat(chatId: number, text: string): Promise<void> {
  const token = process.env.TELEGRAM_BOT_TOKEN;
  if (!token) return;
  const base = process.env.TELEGRAM_API_BASE ?? "https://api.telegram.org";
  try {
    await fetch(`${base}/bot${token}/sendMessage`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ chat_id: chatId, text }),
      signal: AbortSignal.timeout(10_000),
    });
  } catch {
    // Best-effort confirmation; the link is already recorded.
  }
}

export async function POST(request: Request) {
  const secret = process.env.TELEGRAM_WEBHOOK_SECRET;
  if (
    secret &&
    request.headers.get("x-telegram-bot-api-secret-token") !== secret
  ) {
    return new Response(null, { status: 401 });
  }

  let update: TelegramUpdate;
  try {
    update = (await request.json()) as TelegramUpdate;
  } catch {
    return new Response(null, { status: 200 }); // ignore malformed updates
  }

  const text = update.message?.text?.trim() ?? "";
  const chat = update.message?.chat;
  const match = /^\/start\s+([A-Za-z0-9_-]{10,})$/.exec(text);
  if (!match || !chat) {
    return new Response(null, { status: 200 }); // not a connect message
  }
  const token = match[1];

  const admin = createAdminClient();
  const { data, error } = await admin
    .from("telegram_links")
    .update({
      chat_id: String(chat.id),
      chat_name: chat.title ?? chat.username ?? chat.first_name ?? "chat",
    })
    .eq("token", token)
    .is("consumed_at", null)
    .gt("expires_at", new Date().toISOString())
    .select("user_id")
    .maybeSingle();

  if (!error && data) {
    await replyToChat(
      chat.id,
      "✅ Connected to Restorable. Finish setup back in the dashboard — you'll get your backup alerts here.",
    );
  } else {
    await replyToChat(
      chat.id,
      "That connect link is invalid or expired. Generate a new one from the Restorable dashboard.",
    );
  }
  // Always 200 so Telegram doesn't retry.
  return new Response(null, { status: 200 });
}
