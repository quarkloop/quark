import { type NextRequest, NextResponse } from "next/server";

export const dynamic = "force-dynamic";

export async function POST(request: NextRequest) {
  const baseUrl = request.nextUrl.searchParams.get("baseUrl");
  if (!baseUrl) {
    return NextResponse.json({ error: "baseUrl required" }, { status: 400 });
  }
  try {
    const res = await fetch(`${baseUrl}/api/v1/agent/plan/approve`, {
      method: "POST",
    });
    return NextResponse.json(await res.json(), { status: res.status });
  } catch {
    return NextResponse.json({ error: "Agent unreachable" }, { status: 502 });
  }
}
