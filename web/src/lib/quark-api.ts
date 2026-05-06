import { NextResponse } from "next/server";
import type {
  ActivityRecord,
  AgentConnection,
  AgentStatus,
  SessionRecord,
  SessionType,
} from "@/lib/types";
import {
  ACTIVITY_HISTORY_LIMIT,
  DISCOVERY_TIMEOUT_MS,
  SUPERVISOR_PORT_DEFAULT,
} from "@/lib/constants";

export const SUPERVISOR_BASE_URL =
  process.env.QUARK_SUPERVISOR_URL ??
  `http://127.0.0.1:${SUPERVISOR_PORT_DEFAULT}`;

export interface SupervisorRuntime {
  id: string;
  space: string;
  working_dir: string;
  status: "starting" | "running" | "stopping" | "stopped";
  pid?: number;
  port?: number;
  started_at?: string;
  uptime?: string;
}

export interface RuntimeInfo {
  id?: string;
  sessions?: number;
  work_status?: string;
  default_model?: string;
  models?: unknown[];
  channels?: unknown;
}

export interface SupervisorSession {
  id: string;
  space: string;
  type: SessionType;
  title?: string;
  status: string;
  created_at: string;
  updated_at: string;
}

interface RuntimeMessage {
  id?: string;
  role: string;
  content: string;
  timestamp?: string;
}

interface SSEFrame {
  event: string;
  data: string;
}

export function supervisorURL(path: string) {
  return new URL(path, SUPERVISOR_BASE_URL).toString();
}

export function runtimeURL(baseUrl: string, path: string) {
  return new URL(path, normalizeBaseURL(baseUrl)).toString();
}

export function normalizeBaseURL(raw: string) {
  let parsed: URL;
  try {
    parsed = new URL(raw);
  } catch {
    throw new Error("baseUrl must be an absolute URL");
  }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    throw new Error("baseUrl must use http or https");
  }
  parsed.pathname = parsed.pathname.replace(/\/+$/, "");
  parsed.search = "";
  parsed.hash = "";
  return parsed.toString().replace(/\/$/, "");
}

export async function fetchJSON<T>(
  url: string,
  init?: RequestInit,
): Promise<T> {
  const res = await fetch(url, init);
  if (!res.ok) {
    throw new UpstreamError(res.status, await responseError(res));
  }
  return res.json() as Promise<T>;
}

export async function forwardJSON(res: Response) {
  if (res.status === 204) return new NextResponse(null, { status: 204 });
  const text = await res.text();
  if (!text) return new NextResponse(null, { status: res.status });
  try {
    return NextResponse.json(JSON.parse(text), { status: res.status });
  } catch {
    return new NextResponse(text, {
      status: res.status,
      headers: { "Content-Type": "text/plain; charset=utf-8" },
    });
  }
}

export function jsonError(error: string, status = 500) {
  return NextResponse.json({ error }, { status });
}

export function mapUpstreamError(error: unknown, fallback = "Upstream request failed") {
  if (error instanceof UpstreamError) {
    return jsonError(error.message, error.status);
  }
  if (error instanceof Error) {
    return jsonError(error.message, 502);
  }
  return jsonError(fallback, 502);
}

export function runtimeBaseFromPort(port: number) {
  return `http://127.0.0.1:${port}`;
}

export function runtimeStatusToAgentStatus(status: SupervisorRuntime["status"]): AgentStatus {
  switch (status) {
    case "running":
      return "online";
    case "starting":
    case "stopping":
      return "busy";
    case "stopped":
      return "offline";
    default:
      return "unknown";
  }
}

export async function getRuntimeInfo(baseUrl: string): Promise<RuntimeInfo | null> {
  try {
    return await fetchJSON<RuntimeInfo>(runtimeURL(baseUrl, "/v1/info"), {
      signal: AbortSignal.timeout(DISCOVERY_TIMEOUT_MS),
    });
  } catch {
    return null;
  }
}

export async function getRuntimeHealth(baseUrl: string) {
  return fetchJSON<{ status: string }>(runtimeURL(baseUrl, "/v1/health"), {
    signal: AbortSignal.timeout(DISCOVERY_TIMEOUT_MS),
  });
}

export function runtimeToAgent(
  runtime: SupervisorRuntime,
  info: RuntimeInfo | null,
): AgentConnection | null {
  if (!runtime.port) return null;
  const baseUrl = runtimeBaseFromPort(runtime.port);
  const model = info?.default_model;
  return {
    id: runtime.id,
    name: runtime.space || info?.id || `Runtime :${runtime.port}`,
    mode: "proxied",
    baseUrl,
    port: runtime.port,
    status: runtimeStatusToAgentStatus(runtime.status),
    spaceId: runtime.space,
    runtimeId: runtime.id,
    model,
  };
}

export function normalizeSession(session: SupervisorSession): SessionRecord {
  return {
    id: session.id,
    key: session.id,
    agent_id: session.space,
    space: session.space,
    type: session.type,
    status: session.status,
    title: session.title,
    created_at: session.created_at,
    updated_at: session.updated_at,
  };
}

export async function resolveSpace(request: Request, agentId: string) {
  const url = new URL(request.url);
  const explicit = url.searchParams.get("space") ?? url.searchParams.get("spaceId");
  if (explicit) return explicit;

  try {
    const runtime = await fetchJSON<SupervisorRuntime>(
      supervisorURL(`/v1/agents/${encodeURIComponent(agentId)}`),
      { signal: AbortSignal.timeout(DISCOVERY_TIMEOUT_MS) },
    );
    return runtime.space;
  } catch {
    // Manual connections may use a local placeholder ID. Fall back to
    // matching the supplied runtime port against the supervisor registry.
  }

  const baseUrl = url.searchParams.get("baseUrl");
  if (!baseUrl) return null;
  try {
    const port = new URL(normalizeBaseURL(baseUrl)).port;
    if (!port) return null;
    const runtimes = await fetchJSON<SupervisorRuntime[]>(
      supervisorURL("/v1/agents"),
      { signal: AbortSignal.timeout(DISCOVERY_TIMEOUT_MS) },
    );
    return runtimes.find((runtime) => String(runtime.port) === port)?.space ?? null;
  } catch {
    return null;
  }
}

export async function listSessions(space: string) {
  const sessions = await fetchJSON<SupervisorSession[]>(
    supervisorURL(`/v1/spaces/${encodeURIComponent(space)}/sessions`),
  );
  return sessions.map(normalizeSession);
}

export async function listRuntimeMessages(baseUrl: string, sessionId: string) {
  return fetchJSON<RuntimeMessage[]>(
    runtimeURL(baseUrl, `/v1/sessions/${encodeURIComponent(sessionId)}/messages`),
  );
}

export function messagesToActivities(
  messages: RuntimeMessage[],
  sessionId: string,
): ActivityRecord[] {
  return messages.map((message, index) => messageToActivity(message, sessionId, index));
}

export async function sessionActivities(
  baseUrl: string,
  sessionId: string,
  limit = ACTIVITY_HISTORY_LIMIT,
) {
  const messages = await listRuntimeMessages(baseUrl, sessionId);
  return messagesToActivities(messages, sessionId).slice(-limit);
}

export async function aggregateActivities(
  baseUrl: string,
  space: string,
  limit = ACTIVITY_HISTORY_LIMIT,
) {
  const sessions = await listSessions(space);
  const settled = await Promise.allSettled(
    sessions.map((session) => sessionActivities(baseUrl, session.key, limit)),
  );
  return settled
    .flatMap((result) => (result.status === "fulfilled" ? result.value : []))
    .sort((a, b) => a.timestamp.localeCompare(b.timestamp))
    .slice(-limit);
}

export async function waitForRuntimeSession(baseUrl: string, sessionId: string) {
  const deadline = Date.now() + 2_000;
  while (Date.now() < deadline) {
    try {
      await listRuntimeMessages(baseUrl, sessionId);
      return;
    } catch {
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
  }
}

export async function readRuntimeMessageReply(response: Response) {
  if (!response.body) return { reply: "" };

  let reply = "";
  let error = "";
  await readSSE(response.body, (frame) => {
    if (!frame.data) return;
    if (frame.event === "token" || frame.event === "text") {
      const token = parseJSON<string>(frame.data);
      reply += typeof token === "string" ? token : frame.data;
      return;
    }
    if (frame.event === "error") {
      const message = parseJSON<string>(frame.data);
      error = typeof message === "string" ? message : frame.data;
    }
  });
  if (error && !reply) {
    throw new Error(error);
  }
  return { reply };
}

export function streamMessagesAsActivities(body: ReadableStream<Uint8Array>, sessionId: string) {
  const encoder = new TextEncoder();
  let index = 0;

  return transformSSE(body, (frame) => {
    if (!frame.data) return null;
    const message = parseJSON<RuntimeMessage>(frame.data);
    if (!message || typeof message !== "object" || !("role" in message)) {
      return null;
    }
    const activity = messageToActivity(message, sessionId, index++);
    return encoder.encode(`event: message\ndata: ${JSON.stringify(activity)}\n\n`);
  });
}

export class UpstreamError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = "UpstreamError";
  }
}

async function responseError(res: Response) {
  const text = await res.text();
  if (!text) return res.statusText;
  try {
    const parsed = JSON.parse(text) as { error?: string };
    return parsed.error ?? text;
  } catch {
    return text;
  }
}

function messageToActivity(
  message: RuntimeMessage,
  sessionId: string,
  index: number,
): ActivityRecord {
  const timestamp = message.timestamp || new Date().toISOString();
  const author = message.role === "assistant" ? "agent" : message.role;
  return {
    id:
      message.id ||
      `${sessionId}:${message.role}:${timestamp}:${hashString(message.content) || index}`,
    session_id: sessionId,
    type: "message.added",
    timestamp,
    data: {
      author,
      content: message.content,
    },
  };
}

function hashString(value: string) {
  let hash = 0;
  for (let i = 0; i < value.length; i++) {
    hash = (hash * 31 + value.charCodeAt(i)) >>> 0;
  }
  return hash.toString(36);
}

async function readSSE(
  body: ReadableStream<Uint8Array>,
  onFrame: (frame: SSEFrame) => void,
) {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      buffer = drainFrames(buffer, onFrame);
    }
    buffer += decoder.decode();
    drainFrames(buffer, onFrame, true);
  } finally {
    reader.releaseLock();
  }
}

function transformSSE(
  body: ReadableStream<Uint8Array>,
  transform: (frame: SSEFrame) => Uint8Array | null,
) {
  return new ReadableStream<Uint8Array>({
    async start(controller) {
      const reader = body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";

      try {
        for (;;) {
          const { done, value } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          buffer = drainFrames(buffer, (frame) => {
            const chunk = transform(frame);
            if (chunk) controller.enqueue(chunk);
          });
        }
        buffer += decoder.decode();
        drainFrames(
          buffer,
          (frame) => {
            const chunk = transform(frame);
            if (chunk) controller.enqueue(chunk);
          },
          true,
        );
        controller.close();
      } catch (error) {
        controller.error(error);
      } finally {
        reader.releaseLock();
      }
    },
  });
}

function drainFrames(
  input: string,
  onFrame: (frame: SSEFrame) => void,
  flush = false,
) {
  const normalized = input.replace(/\r\n/g, "\n");
  const frames = normalized.split("\n\n");
  const remainder = flush ? "" : (frames.pop() ?? "");
  for (const raw of frames) {
    const frame = parseSSEFrame(raw);
    if (frame) onFrame(frame);
  }
  if (flush && remainder.trim()) {
    const frame = parseSSEFrame(remainder);
    if (frame) onFrame(frame);
  }
  return remainder;
}

function parseSSEFrame(raw: string): SSEFrame | null {
  let event = "message";
  const data: string[] = [];
  for (const line of raw.split("\n")) {
    if (!line || line.startsWith(":")) continue;
    if (line.startsWith("event:")) {
      event = line.slice(6).trim();
      continue;
    }
    if (line.startsWith("data:")) {
      data.push(line.slice(5).trimStart());
    }
  }
  if (data.length === 0) return null;
  return { event, data: data.join("\n") };
}

function parseJSON<T>(value: string): T | null {
  try {
    return JSON.parse(value) as T;
  } catch {
    return null;
  }
}
