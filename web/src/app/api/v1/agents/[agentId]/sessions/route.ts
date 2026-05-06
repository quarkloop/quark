import { type NextRequest, NextResponse } from "next/server";
import {
  forwardJSON,
  jsonError,
  mapUpstreamError,
  normalizeSession,
  resolveSpace,
  supervisorURL,
  waitForRuntimeSession,
} from "@/lib/quark-api";

export const dynamic = "force-dynamic";

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ agentId: string }> },
) {
  const { agentId } = await params;
  const space = await resolveSpace(request, agentId);
  if (!space) return jsonError("spaceId required", 400);

  try {
    const res = await fetch(
      supervisorURL(`/v1/spaces/${encodeURIComponent(space)}/sessions`),
    );
    const sessions = await res.json();
    if (!res.ok) return NextResponse.json(sessions, { status: res.status });
    return NextResponse.json(sessions.map(normalizeSession), { status: res.status });
  } catch (error) {
    return mapUpstreamError(error, "Supervisor unreachable");
  }
}

export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ agentId: string }> },
) {
  const { agentId } = await params;
  const baseUrl = request.nextUrl.searchParams.get("baseUrl");
  const space = await resolveSpace(request, agentId);
  if (!space) return jsonError("spaceId required", 400);

  try {
    const body = await request.text();
    const res = await fetch(
      supervisorURL(`/v1/spaces/${encodeURIComponent(space)}/sessions`),
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body,
      },
    );
    if (!res.ok) return forwardJSON(res);

    const session = normalizeSession(await res.json());
    if (baseUrl) {
      await waitForRuntimeSession(baseUrl, session.key).catch(() => {});
    }
    return NextResponse.json({ session }, { status: res.status });
  } catch (error) {
    return mapUpstreamError(error, "Supervisor unreachable");
  }
}
