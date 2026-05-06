import { type NextRequest, NextResponse } from "next/server";
import { jsonError, mapUpstreamError, sessionActivities } from "@/lib/quark-api";

export const dynamic = "force-dynamic";

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ sessionKey: string }> },
) {
  const { sessionKey } = await params;
  const baseUrl = request.nextUrl.searchParams.get("baseUrl");
  const limit = Number(request.nextUrl.searchParams.get("limit") || "128");
  if (!baseUrl) return jsonError("baseUrl required", 400);

  try {
    const activities = await sessionActivities(baseUrl, sessionKey, limit);
    return NextResponse.json(activities);
  } catch (error) {
    return mapUpstreamError(error, "Runtime unreachable");
  }
}
