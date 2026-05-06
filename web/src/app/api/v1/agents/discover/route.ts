import { NextResponse } from "next/server";
import type { AgentConnection } from "@/lib/types";
import { DISCOVERY_TIMEOUT_MS } from "@/lib/constants";
import {
  fetchJSON,
  getRuntimeInfo,
  runtimeBaseFromPort,
  runtimeToAgent,
  supervisorURL,
  type SupervisorRuntime,
} from "@/lib/quark-api";

export const dynamic = "force-dynamic";

export async function GET() {
  const agents: AgentConnection[] = [];

  try {
    const runtimes = await fetchJSON<SupervisorRuntime[]>(
      supervisorURL("/v1/agents"),
      { signal: AbortSignal.timeout(DISCOVERY_TIMEOUT_MS) },
    );

    const managed = await Promise.all(
      runtimes.map(async (runtime) => {
        const baseUrl = runtime.port ? runtimeBaseFromPort(runtime.port) : "";
        const info = baseUrl ? await getRuntimeInfo(baseUrl) : null;
        return runtimeToAgent(runtime, info);
      }),
    );
    for (const agent of managed) {
      if (agent) agents.push(agent);
    }
  } catch {
    // Supervisor may not be running while the user is editing the web app.
  }

  agents.sort((a, b) => a.port - b.port);
  return NextResponse.json(agents);
}
