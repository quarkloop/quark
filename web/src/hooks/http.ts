// ─── HTTP helpers ────────────────────────────────────────────────

export const BASE = (baseUrl: string, spaceId?: string) => {
  const params = new URLSearchParams({ baseUrl });
  if (spaceId) params.set("spaceId", spaceId);
  return params.toString();
};

export async function get<T>(path: string): Promise<T> {
  const res = await fetch(path);
  if (!res.ok) throw new Error(`GET ${path} failed`);
  return res.json();
}

export async function post<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method: "POST",
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

export async function del(path: string): Promise<void> {
  const res = await fetch(path, { method: "DELETE" });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
}

// ─── Query keys ─────────────────────────────────────────────────

export function agentKey(agentId: string, path: string) {
  return ["agents", agentId, path] as const;
}

export const AGENTS_KEY = ["agents"] as const;
export const DISABLED_KEY = ["disabled"] as const;
