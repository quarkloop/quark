import { type NextRequest, NextResponse } from "next/server";
import {
  aggregateActivities,
  jsonError,
  mapUpstreamError,
  resolveSpace,
} from "@/lib/quark-api";

export const dynamic = "force-dynamic";

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ agentId: string }> },
) {
  const { agentId } = await params;
  const baseUrl = request.nextUrl.searchParams.get("baseUrl");
  const limit = Number(request.nextUrl.searchParams.get("limit") || "128");
  if (!baseUrl) return jsonError("baseUrl required", 400);

  const space = await resolveSpace(request, agentId);
  if (!space) return jsonError("spaceId required", 400);

  try {
    const activities = await aggregateActivities(baseUrl, space, limit);
    return NextResponse.json(activities);
  } catch (error) {
    return mapUpstreamError(error, "Runtime unreachable");
  }
}
