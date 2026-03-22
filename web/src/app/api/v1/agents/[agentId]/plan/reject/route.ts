import { type NextRequest, NextResponse } from "next/server";

export const dynamic = "force-dynamic";

export async function POST(request: NextRequest) {
  const baseUrl = request.nextUrl.searchParams.get("baseUrl");
  if (!baseUrl) {
    return NextResponse.json({ error: "baseUrl required" }, { status: 400 });
  }
  try {
    const res = await fetch(`${baseUrl}/api/v1/agent/plan/reject`, {
      method: "POST",
    });
    if (res.status === 204) {
      return new NextResponse(null, { status: 204 });
    }
    return NextResponse.json(await res.json(), { status: res.status });
  } catch {
    return NextResponse.json({ error: "Agent unreachable" }, { status: 502 });
  }
}
