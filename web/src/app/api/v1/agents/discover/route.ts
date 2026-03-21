import { NextResponse } from "next/server";
import type { AgentConnection } from "@/lib/types";
import {
  API_SERVER_PORT,
  AGENT_PORT_RANGE_START,
  AGENT_PORT_RANGE_END,
  DISCOVERY_TIMEOUT_MS,
} from "@/lib/constants";

export const dynamic = "force-dynamic";

export async function GET() {
  const agents: AgentConnection[] = [];
  const seen = new Set<number>();

  // 1. Check if api-server is running and enumerate managed spaces.
  try {
    const res = await fetch(
      `http://127.0.0.1:${API_SERVER_PORT}/api/v1/spaces`,
      { signal: AbortSignal.timeout(DISCOVERY_TIMEOUT_MS) },
    );
    if (res.ok) {
      const spaces = await res.json();
      for (const sp of spaces) {
        if (sp.status === "running" && sp.port) {
          seen.add(sp.port);
          agents.push({
            id: sp.id,
            name: sp.name || sp.id,
            mode: "proxied",
            baseUrl: `http://127.0.0.1:${sp.port}`,
            port: sp.port,
            status: "online",
            spaceId: sp.id,
          });
        }
      }
    }
  } catch {
    // api-server not running — continue with direct scan.
  }

  // 2. Scan port range for direct agents.
  const probes: Promise<void>[] = [];
  for (let port = AGENT_PORT_RANGE_START; port <= AGENT_PORT_RANGE_END; port++) {
    if (seen.has(port)) continue;
    probes.push(
      probeAgent(port).then((agent) => {
        if (agent) agents.push(agent);
      }),
    );
  }
  await Promise.allSettled(probes);

  // Sort by port.
  agents.sort((a, b) => a.port - b.port);
  return NextResponse.json(agents);
}

async function probeAgent(port: number): Promise<AgentConnection | null> {
  try {
    // Use /info for full metadata (provider, model, tools).
    const res = await fetch(
      `http://127.0.0.1:${port}/api/v1/agent/info`,
      { signal: AbortSignal.timeout(DISCOVERY_TIMEOUT_MS) },
    );
    if (!res.ok) return null;
    const data = await res.json();
    return {
      id: data.agent_id || `agent-${port}`,
      name: data.agent_id || `Agent :${port}`,
      mode: "direct",
      baseUrl: `http://127.0.0.1:${port}`,
      port,
      status: "online",
      provider: data.provider,
      model: data.model,
    };
  } catch {
    return null;
  }
}
