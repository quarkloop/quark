import { type NextRequest, NextResponse } from "next/server";
import { CHAT_TIMEOUT_MS } from "@/lib/constants";

export const dynamic = "force-dynamic";

export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ agentId: string }> },
) {
  await params;
  const baseUrl = request.nextUrl.searchParams.get("baseUrl");
  if (!baseUrl) {
    return NextResponse.json(
      { error: "baseUrl query param required" },
      { status: 400 },
    );
  }

  const ct = request.headers.get("content-type") || "";
  const targetUrl = `${baseUrl}/api/v1/agent/chat`;

  try {
    let res: Response;

    if (ct.includes("multipart/form-data")) {
      // Forward multipart body to the agent.
      const form = await request.formData();
      res = await fetch(targetUrl, {
        method: "POST",
        body: form,
        signal: AbortSignal.timeout(CHAT_TIMEOUT_MS),
      });
    } else {
      // Forward JSON body.
      const body = await request.text();
      res = await fetch(targetUrl, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body,
        signal: AbortSignal.timeout(CHAT_TIMEOUT_MS),
      });
    }

    const data = await res.json();
    return NextResponse.json(data, { status: res.status });
  } catch (err) {
    const message = err instanceof Error ? err.message : "Request failed";
    return NextResponse.json({ error: message }, { status: 502 });
  }
}
