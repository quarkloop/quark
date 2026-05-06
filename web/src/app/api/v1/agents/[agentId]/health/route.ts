import { type NextRequest, NextResponse } from "next/server";
import { DISCOVERY_TIMEOUT_MS } from "@/lib/constants";
import { runtimeURL } from "@/lib/quark-api";

export const dynamic = "force-dynamic";

export async function GET(request: NextRequest) {
  const baseUrl = request.nextUrl.searchParams.get("baseUrl");
  if (!baseUrl) {
    return NextResponse.json({ error: "baseUrl required" }, { status: 400 });
  }
  try {
    const res = await fetch(runtimeURL(baseUrl, "/v1/health"), {
      signal: AbortSignal.timeout(DISCOVERY_TIMEOUT_MS),
    });
    return NextResponse.json(await res.json(), { status: res.status });
  } catch (error) {
    const message = error instanceof Error ? error.message : "Runtime unreachable";
    return NextResponse.json({ error: message }, { status: 502 });
  }
}
