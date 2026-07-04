import { createClient } from "@/lib/supabase/server";
import type { Agent } from "@/lib/types";

import { NewAgentToken } from "./new-agent-token";

export const dynamic = "force-dynamic";

export default async function AgentsPage() {
  const supabase = await createClient();
  const { data: agents } = await supabase
    .from("agents")
    .select("id, name, agent_version, last_seen, created_at")
    .order("created_at", { ascending: true })
    .returns<Agent[]>();

  return (
    <main>
      <h1 className="mb-1 text-xl font-semibold">Agents</h1>
      <p className="mb-6 text-sm text-neutral-500">
        Agents run in your homelab and connect outbound only — no inbound ports, ever.
      </p>

      <NewAgentToken />

      {!agents?.length ? (
        <p className="mt-8 text-sm text-neutral-500">No agents registered yet.</p>
      ) : (
        <table className="mt-8 w-full text-left text-sm">
          <thead>
            <tr className="border-b border-neutral-200 text-neutral-500 dark:border-neutral-800">
              <th className="py-2 font-normal">Name</th>
              <th className="py-2 font-normal">Version</th>
              <th className="py-2 font-normal">Last seen</th>
            </tr>
          </thead>
          <tbody>
            {agents.map((agent) => (
              <tr key={agent.id} className="border-b border-neutral-100 dark:border-neutral-900">
                <td className="py-3">{agent.name}</td>
                <td className="py-3 text-neutral-500">{agent.agent_version ?? "—"}</td>
                <td className="py-3 text-neutral-500">
                  {agent.last_seen ? new Date(agent.last_seen).toLocaleString() : "never"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </main>
  );
}
