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
  SessionRecord,
  CreateSessionRequest,
  CreateSessionResponse,
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
  sessionKey?: string,
): Promise<ChatResponse> {
  const url = `/api/v1/agents/${agentId}/chat?baseUrl=${encodeURIComponent(baseUrl)}`;

  if (files && files.length > 0) {
    const form = new FormData();
    form.append("message", message);
    form.append("mode", mode);
    if (sessionKey) form.append("session_key", sessionKey);
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
    body: JSON.stringify({ message, mode, session_key: sessionKey }),
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

export async function approvePlan(
  agentId: string,
  baseUrl: string,
): Promise<Plan> {
  const res = await fetch(
    `/api/v1/agents/${agentId}/plan/approve?baseUrl=${encodeURIComponent(baseUrl)}`,
    { method: "POST" },
  );
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
  return res.json();
}

export async function rejectPlan(
  agentId: string,
  baseUrl: string,
): Promise<void> {
  const res = await fetch(
    `/api/v1/agents/${agentId}/plan/reject?baseUrl=${encodeURIComponent(baseUrl)}`,
    { method: "POST" },
  );
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
}

export async function getSessions(
  agentId: string,
  baseUrl: string,
): Promise<SessionRecord[]> {
  const res = await fetch(
    `/api/v1/agents/${agentId}/sessions?baseUrl=${encodeURIComponent(baseUrl)}`,
  );
  if (!res.ok) throw new Error("Sessions fetch failed");
  return res.json();
}

export async function getSessionActivity(
  agentId: string,
  baseUrl: string,
  sessionKey: string,
  limit = 128,
): Promise<ActivityRecord[]> {
  const res = await fetch(
    `/api/v1/agents/${agentId}/sessions/${encodeURIComponent(sessionKey)}/activity?baseUrl=${encodeURIComponent(baseUrl)}&limit=${limit}`,
  );
  if (!res.ok) throw new Error("Session activity fetch failed");
  return res.json();
}

export async function createSession(
  agentId: string,
  baseUrl: string,
  req: CreateSessionRequest,
): Promise<CreateSessionResponse> {
  const res = await fetch(
    `/api/v1/agents/${agentId}/sessions?baseUrl=${encodeURIComponent(baseUrl)}`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(req),
    },
  );
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
  return res.json();
}

export async function deleteSession(
  agentId: string,
  baseUrl: string,
  sessionKey: string,
): Promise<void> {
  const res = await fetch(
    `/api/v1/agents/${agentId}/sessions/${encodeURIComponent(sessionKey)}?baseUrl=${encodeURIComponent(baseUrl)}`,
    { method: "DELETE" },
  );
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
}
