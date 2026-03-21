import { type NextRequest, NextResponse } from "next/server";
import { DISCOVERY_TIMEOUT_MS } from "@/lib/constants";

export const dynamic = "force-dynamic";

export async function GET(request: NextRequest) {
  const baseUrl = request.nextUrl.searchParams.get("baseUrl");
  if (!baseUrl) {
    return NextResponse.json({ error: "baseUrl required" }, { status: 400 });
  }
  try {
    const res = await fetch(`${baseUrl}/api/v1/agent/health`, {
      signal: AbortSignal.timeout(DISCOVERY_TIMEOUT_MS),
    });
    return NextResponse.json(await res.json(), { status: res.status });
  } catch {
    return NextResponse.json({ error: "Agent unreachable" }, { status: 502 });
  }
}
