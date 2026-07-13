"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState, useTransition } from "react";

import type { ChannelType } from "@/lib/alerts/config";

import {
  createAlertChannel,
  deleteAlertChannel,
  pollTelegramConnect,
  startTelegramConnect,
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

export function ChannelsManager({
  channels,
  atLimit,
}: {
  channels: ChannelRow[];
  atLimit: boolean;
}) {
  const router = useRouter();
  const [adding, setAdding] = useState(false);
  const [type, setType] = useState<ChannelType>("telegram");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [tgUrl, setTgUrl] = useState<string | null>(null);
  const [connecting, setConnecting] = useState(false);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const [pending, startTransition] = useTransition();

  useEffect(() => {
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, []);

  function submit(formData: FormData) {
    startTransition(async () => {
      setError(null);
      const result = await createAlertChannel(null, formData);
      if (result.error) {
        setError(result.error);
        return;
      }
      setAdding(false);
      setNotice("Channel added — press Test to verify it before alerts flow.");
    });
  }

  // Telegram: mint a one-time deep link, open it, then poll until the bot
  // webhook reports the user's chat_id and the channel is created.
  function connectTelegram() {
    startTransition(async () => {
      setError(null);
      setNotice(null);
      const result = await startTelegramConnect();
      if (result.error || !result.url || !result.token) {
        setError(result.error ?? "could not start Telegram connect");
        return;
      }
      setTgUrl(result.url);
      setConnecting(true);
      window.open(result.url, "_blank", "noopener");
      const token = result.token;
      const started = Date.now();
      pollRef.current = setInterval(async () => {
        if (Date.now() - started > 3 * 60_000) {
          if (pollRef.current) clearInterval(pollRef.current);
          setConnecting(false);
          setError("Timed out waiting for Telegram. Tap the link and press Start, then try again.");
          return;
        }
        const poll = await pollTelegramConnect(token);
        if (poll.error) {
          if (pollRef.current) clearInterval(pollRef.current);
          setConnecting(false);
          setError(poll.error);
        } else if (poll.connected) {
          if (pollRef.current) clearInterval(pollRef.current);
          setConnecting(false);
          setAdding(false);
          setTgUrl(null);
          setNotice("Telegram connected — you'll get alerts there.");
          router.refresh();
        }
      }, 2000);
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


  return (
    <section>
      <div className="flex items-center justify-between">
        <h2 className="font-medium">Channels</h2>
        {!adding && !atLimit && (
          <button
            onClick={() => setAdding(true)}
            className="rounded bg-neutral-900 px-3 py-1.5 text-sm text-white dark:bg-white dark:text-neutral-900"
          >
            Add channel
          </button>
        )}
      </div>

      {atLimit && (
        <div className="mt-3 flex items-center justify-between gap-3 rounded border border-neutral-200 bg-neutral-50 p-3 text-sm dark:border-neutral-800 dark:bg-neutral-900">
          <span className="text-neutral-600 dark:text-neutral-400">
            You&apos;re on the free plan&apos;s one-channel limit. Upgrade to Pro for all channels.
          </span>
          <Link
            href="/dashboard/billing"
            className="shrink-0 rounded bg-neutral-900 px-3 py-1.5 text-white dark:bg-white dark:text-neutral-900"
          >
            Upgrade
          </Link>
        </div>
      )}

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
            <div className="flex flex-col gap-2">
              <p className="text-sm text-neutral-600 dark:text-neutral-400">
                Connect your Telegram in one tap — no chat IDs, no bot setup.
                We open our bot; press <strong>Start</strong> and you&apos;re done.
              </p>
              <button
                type="button"
                onClick={connectTelegram}
                disabled={pending || connecting}
                className="w-fit rounded bg-[#229ED9] px-3 py-1.5 text-white disabled:opacity-50"
              >
                {connecting ? "Waiting for Telegram…" : "Connect with Telegram"}
              </button>
              {tgUrl && connecting && (
                <p className="text-xs text-neutral-500">
                  Didn&apos;t open?{" "}
                  <a href={tgUrl} target="_blank" rel="noopener" className="underline">
                    Tap here
                  </a>{" "}
                  and press Start in Telegram.
                </p>
              )}
            </div>
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
            {type !== "telegram" && (
              <button
                type="submit"
                disabled={pending}
                className="rounded bg-neutral-900 px-3 py-1.5 text-white disabled:opacity-50 dark:bg-white dark:text-neutral-900"
              >
                Save
              </button>
            )}
            <button
              type="button"
              onClick={() => {
                setAdding(false);
                setError(null);
                setConnecting(false);
                setTgUrl(null);
                if (pollRef.current) clearInterval(pollRef.current);
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
