"use client";

import { useState, useTransition } from "react";

import type { ChannelType } from "@/lib/alerts/config";

import {
  createAlertChannel,
  deleteAlertChannel,
  detectTelegramChats,
  testAlertChannel,
} from "./actions";

export interface ChannelRow {
  id: string;
  type: ChannelType;
  config: Record<string, string>;
  verified: boolean;
  created_at: string;
}

const typeLabels: Record<ChannelType, string> = {
  telegram: "Telegram",
  discord: "Discord webhook",
  ntfy: "ntfy",
  email: "Email",
};

function describe(channel: ChannelRow): string {
  switch (channel.type) {
    case "telegram":
      return `chat ${channel.config.chat_id}`;
    case "discord":
      return "webhook …" + (channel.config.webhook_url ?? "").slice(-18);
    case "ntfy":
      return `${channel.config.server ?? "https://ntfy.sh"}/${channel.config.topic}`;
    case "email":
      return channel.config.to ?? "";
  }
}

export function ChannelsManager({ channels }: { channels: ChannelRow[] }) {
  const [adding, setAdding] = useState(false);
  const [type, setType] = useState<ChannelType>("telegram");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [chats, setChats] = useState<{ id: string; name: string }[]>([]);
  const [pending, startTransition] = useTransition();

  function submit(formData: FormData) {
    startTransition(async () => {
      setError(null);
      const result = await createAlertChannel(null, formData);
      if (result.error) {
        setError(result.error);
        return;
      }
      setAdding(false);
      setChats([]);
      setNotice("Channel added — press Test to verify it before alerts flow.");
    });
  }

  function test(id: string) {
    startTransition(async () => {
      setError(null);
      setNotice(null);
      const result = await testAlertChannel(id);
      if (result.error) {
        setError(`Test failed: ${result.error}`);
      } else {
        setNotice("Test message sent — channel verified.");
      }
    });
  }

  function remove(id: string) {
    startTransition(async () => {
      await deleteAlertChannel(id);
    });
  }

  function findChats() {
    startTransition(async () => {
      setError(null);
      const result = await detectTelegramChats();
      if (result.error) {
        setError(result.error);
        return;
      }
      setChats(result.chats ?? []);
    });
  }

  return (
    <section>
      <div className="flex items-center justify-between">
        <h2 className="font-medium">Channels</h2>
        {!adding && (
          <button
            onClick={() => setAdding(true)}
            className="rounded bg-neutral-900 px-3 py-1.5 text-sm text-white dark:bg-white dark:text-neutral-900"
          >
            Add channel
          </button>
        )}
      </div>

      {notice && <p className="mt-2 text-sm text-green-700 dark:text-green-400">{notice}</p>}
      {error && <p className="mt-2 text-sm text-red-600">{error}</p>}

      {adding && (
        <form
          action={submit}
          className="mt-4 flex flex-col gap-3 rounded border border-neutral-200 p-4 text-sm dark:border-neutral-800"
        >
          <label className="flex flex-col gap-1">
            Type
            <select
              name="type"
              value={type}
              onChange={(e) => setType(e.target.value as ChannelType)}
              className="w-fit rounded border border-neutral-300 px-2 py-1.5 dark:border-neutral-700 dark:bg-neutral-900"
            >
              {Object.entries(typeLabels).map(([value, label]) => (
                <option key={value} value={value}>
                  {label}
                </option>
              ))}
            </select>
          </label>

          {type === "telegram" && (
            <>
              <label className="flex flex-col gap-1">
                Chat ID
                <input
                  name="chat_id"
                  required
                  placeholder="123456789"
                  className="rounded border border-neutral-300 px-2 py-1.5 font-mono dark:border-neutral-700 dark:bg-neutral-900"
                />
              </label>
              <div className="text-xs text-neutral-500">
                Message the bot on Telegram first, then{" "}
                <button type="button" onClick={findChats} className="underline">
                  detect my chat ID
                </button>
                {chats.length > 0 && (
                  <span> — found: {chats.map((c) => `${c.name} (${c.id})`).join(", ")}</span>
                )}
              </div>
            </>
          )}
          {type === "discord" && (
            <label className="flex flex-col gap-1">
              Webhook URL
              <input
                name="webhook_url"
                required
                placeholder="https://discord.com/api/webhooks/…"
                className="rounded border border-neutral-300 px-2 py-1.5 font-mono dark:border-neutral-700 dark:bg-neutral-900"
              />
            </label>
          )}
          {type === "ntfy" && (
            <>
              <label className="flex flex-col gap-1">
                Topic
                <input
                  name="topic"
                  required
                  placeholder="my-backups"
                  className="rounded border border-neutral-300 px-2 py-1.5 font-mono dark:border-neutral-700 dark:bg-neutral-900"
                />
              </label>
              <label className="flex flex-col gap-1">
                Server (optional, default ntfy.sh)
                <input
                  name="server"
                  placeholder="https://ntfy.example.com"
                  className="rounded border border-neutral-300 px-2 py-1.5 font-mono dark:border-neutral-700 dark:bg-neutral-900"
                />
              </label>
            </>
          )}
          {type === "email" && (
            <label className="flex flex-col gap-1">
              Email address
              <input
                name="to"
                type="email"
                required
                className="rounded border border-neutral-300 px-2 py-1.5 dark:border-neutral-700 dark:bg-neutral-900"
              />
            </label>
          )}

          <div className="flex gap-2">
            <button
              type="submit"
              disabled={pending}
              className="rounded bg-neutral-900 px-3 py-1.5 text-white disabled:opacity-50 dark:bg-white dark:text-neutral-900"
            >
              Save
            </button>
            <button
              type="button"
              onClick={() => {
                setAdding(false);
                setError(null);
              }}
              className="rounded border border-neutral-300 px-3 py-1.5 dark:border-neutral-700"
            >
              Cancel
            </button>
          </div>
        </form>
      )}

      {channels.length > 0 && (
        <ul className="mt-4 flex flex-col gap-2">
          {channels.map((channel) => (
            <li
              key={channel.id}
              className="flex items-center gap-3 rounded border border-neutral-200 p-3 text-sm dark:border-neutral-800"
            >
              <span className="font-medium">{typeLabels[channel.type]}</span>
              <span className="text-neutral-500">{describe(channel)}</span>
              {channel.verified ? (
                <span className="rounded bg-green-100 px-2 py-0.5 text-xs text-green-800 dark:bg-green-900/40 dark:text-green-300">
                  verified
                </span>
              ) : (
                <span className="rounded bg-amber-100 px-2 py-0.5 text-xs text-amber-800 dark:bg-amber-900/40 dark:text-amber-300">
                  unverified — press Test
                </span>
              )}
              <div className="ml-auto flex gap-2">
                <button
                  onClick={() => test(channel.id)}
                  disabled={pending}
                  className="rounded border border-neutral-300 px-2 py-1 text-xs disabled:opacity-50 dark:border-neutral-700"
                >
                  Test
                </button>
                <button
                  onClick={() => remove(channel.id)}
                  disabled={pending}
                  className="rounded border border-neutral-300 px-2 py-1 text-xs text-red-600 disabled:opacity-50 dark:border-neutral-700"
                >
                  Delete
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
