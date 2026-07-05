import { createClient } from "@/lib/supabase/server";

import { ChannelsManager, type ChannelRow } from "./channels-manager";

export const dynamic = "force-dynamic";

interface AlertRow {
  id: string;
  kind: string;
  status: string;
  message: string;
  fired_at: string;
  resolved_at: string | null;
}

export default async function AlertsPage() {
  const supabase = await createClient();
  const { data: channels } = await supabase
    .from("alert_channels")
    .select("id, type, config, verified, created_at")
    .order("created_at", { ascending: true })
    .returns<ChannelRow[]>();
  const { data: alerts } = await supabase
    .from("alerts")
    .select("id, kind, status, message, fired_at, resolved_at")
    .order("fired_at", { ascending: false })
    .limit(50)
    .returns<AlertRow[]>();

  return (
    <main>
      <h1 className="mb-1 text-xl font-semibold">Alerts</h1>
      <p className="mb-6 text-sm text-neutral-500">
        Get told when a restore test fails, a backup goes quiet, or an agent
        goes silent — and when things recover.
      </p>

      <ChannelsManager channels={channels ?? []} />

      <h2 className="mb-3 mt-10 font-medium">Recent alerts</h2>
      {!alerts?.length ? (
        <p className="text-sm text-neutral-500">
          No alerts yet. That is a good thing.
        </p>
      ) : (
        <ul className="flex flex-col gap-2">
          {alerts.map((alert) => (
            <li
              key={alert.id}
              className="rounded border border-neutral-200 p-3 text-sm dark:border-neutral-800"
            >
              <div className="flex items-center gap-2">
                <span
                  className={`inline-block rounded px-2 py-0.5 text-xs font-medium uppercase ${
                    alert.status === "firing"
                      ? "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300"
                      : "bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300"
                  }`}
                >
                  {alert.status}
                </span>
                <span className="text-xs text-neutral-500">{alert.kind}</span>
                <span className="ml-auto text-xs text-neutral-500">
                  {new Date(alert.fired_at).toLocaleString()}
                </span>
              </div>
              <p className="mt-2 text-neutral-600 dark:text-neutral-400">{alert.message}</p>
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}
