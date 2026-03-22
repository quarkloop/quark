import { type NextRequest, NextResponse } from "next/server";

export const dynamic = "force-dynamic";

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ sessionKey: string }> },
) {
  const { sessionKey: sessionId } = await params;
  const baseUrl = request.nextUrl.searchParams.get("baseUrl");
  const limit = request.nextUrl.searchParams.get("limit") || "128";
  if (!baseUrl) {
    return NextResponse.json({ error: "baseUrl required" }, { status: 400 });
  }
  try {
    const res = await fetch(
      `${baseUrl}/api/v1/agent/sessions/${sessionId}/activity?limit=${limit}`,
    );
    return NextResponse.json(await res.json(), { status: res.status });
  } catch {
    return NextResponse.json({ error: "Agent unreachable" }, { status: 502 });
  }
}
