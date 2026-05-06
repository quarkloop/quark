import { type NextRequest, NextResponse } from "next/server";
import { runtimeURL } from "@/lib/quark-api";

export const dynamic = "force-dynamic";

export async function GET(request: NextRequest) {
  const baseUrl = request.nextUrl.searchParams.get("baseUrl");
  if (!baseUrl) {
    return NextResponse.json({ error: "baseUrl required" }, { status: 400 });
  }
  try {
    const res = await fetch(runtimeURL(baseUrl, "/v1/info"));
    const info = await res.json();
    return NextResponse.json(
      { mode: info.work_status ?? "autonomous" },
      { status: res.status },
    );
  } catch (error) {
    const message = error instanceof Error ? error.message : "Runtime unreachable";
    return NextResponse.json({ error: message }, { status: 502 });
  }
}
