import { type NextRequest, NextResponse } from "next/server";
import {
  forwardJSON,
  jsonError,
  mapUpstreamError,
  normalizeSession,
  resolveSpace,
  supervisorURL,
} from "@/lib/quark-api";

export const dynamic = "force-dynamic";

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ agentId: string; sessionKey: string }> },
) {
  const { agentId, sessionKey } = await params;
  const space = await resolveSpace(request, agentId);
  if (!space) return jsonError("spaceId required", 400);

  try {
    const res = await fetch(
      supervisorURL(
        `/v1/spaces/${encodeURIComponent(space)}/sessions/${encodeURIComponent(sessionKey)}`,
      ),
    );
    if (!res.ok) return forwardJSON(res);
    return NextResponse.json(normalizeSession(await res.json()), {
      status: res.status,
    });
  } catch (error) {
    return mapUpstreamError(error, "Supervisor unreachable");
  }
}

export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ agentId: string; sessionKey: string }> },
) {
  const { agentId, sessionKey } = await params;
  const space = await resolveSpace(request, agentId);
  if (!space) return jsonError("spaceId required", 400);

  try {
    const res = await fetch(
      supervisorURL(
        `/v1/spaces/${encodeURIComponent(space)}/sessions/${encodeURIComponent(sessionKey)}`,
      ),
      { method: "DELETE" },
    );
    return forwardJSON(res);
  } catch (error) {
    return mapUpstreamError(error, "Supervisor unreachable");
  }
}
