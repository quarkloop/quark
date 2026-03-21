import type {
  AgentConnection,
  AgentMode,
  ChatResponse,
  HealthResponse,
  ActivityRecord,
  StatsResponse,
  Plan,
  ModeResponse,
  FileAttachment,
} from "./types";
import { CHAT_TIMEOUT_MS, ACTIVITY_HISTORY_LIMIT } from "./constants";

export async function discoverAgents(): Promise<AgentConnection[]> {
  const res = await fetch("/api/v1/agents/discover");
  if (!res.ok) throw new Error("Discovery failed");
  return res.json();
}

export async function sendMessage(
  agentId: string,
  baseUrl: string,
  message: string,
  mode: AgentMode = "ask",
  files?: FileAttachment[],
): Promise<ChatResponse> {
  const url = `/api/v1/agents/${agentId}/chat?baseUrl=${encodeURIComponent(baseUrl)}`;

  if (files && files.length > 0) {
    const form = new FormData();
    form.append("message", message);
    form.append("mode", mode);
    for (const f of files) {
      form.append("files", f.file, f.name);
    }
    const res = await fetch(url, {
      method: "POST",
      body: form,
      signal: AbortSignal.timeout(CHAT_TIMEOUT_MS),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }));
      throw new Error(err.error || res.statusText);
    }
    return res.json();
  }

  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ message, mode }),
    signal: AbortSignal.timeout(CHAT_TIMEOUT_MS),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
  return res.json();
}

export async function getHealth(
  agentId: string,
  baseUrl: string,
): Promise<HealthResponse> {
  const res = await fetch(
    `/api/v1/agents/${agentId}/health?baseUrl=${encodeURIComponent(baseUrl)}`,
  );
  if (!res.ok) throw new Error("Health check failed");
  return res.json();
}

export async function getActivity(
  agentId: string,
  baseUrl: string,
  limit = ACTIVITY_HISTORY_LIMIT,
): Promise<ActivityRecord[]> {
  const res = await fetch(
    `/api/v1/agents/${agentId}/activity?baseUrl=${encodeURIComponent(baseUrl)}&limit=${limit}`,
  );
  if (!res.ok) throw new Error("Activity fetch failed");
  return res.json();
}

export function createActivityStream(
  agentId: string,
  baseUrl: string,
): EventSource {
  return new EventSource(
    `/api/v1/agents/${agentId}/activity/stream?baseUrl=${encodeURIComponent(baseUrl)}`,
  );
}

export async function getStats(
  agentId: string,
  baseUrl: string,
): Promise<StatsResponse> {
  const res = await fetch(
    `/api/v1/agents/${agentId}/stats?baseUrl=${encodeURIComponent(baseUrl)}`,
  );
  if (!res.ok) throw new Error("Stats fetch failed");
  return res.json();
}

export async function getMode(
  agentId: string,
  baseUrl: string,
): Promise<ModeResponse> {
  const res = await fetch(
    `/api/v1/agents/${agentId}/mode?baseUrl=${encodeURIComponent(baseUrl)}`,
  );
  if (!res.ok) throw new Error("Mode fetch failed");
  return res.json();
}

export async function getPlan(
  agentId: string,
  baseUrl: string,
): Promise<Plan> {
  const res = await fetch(
    `/api/v1/agents/${agentId}/plan?baseUrl=${encodeURIComponent(baseUrl)}`,
  );
  if (!res.ok) throw new Error("Plan fetch failed");
  return res.json();
}
