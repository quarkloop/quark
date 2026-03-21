import { type NextRequest, NextResponse } from "next/server";

export const dynamic = "force-dynamic";

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ agentId: string }> },
) {
  const { agentId: _ } = await params;
  const baseUrl = request.nextUrl.searchParams.get("baseUrl");
  if (!baseUrl) {
    return NextResponse.json(
      { error: "baseUrl query param required" },
      { status: 400 },
    );
  }

  const targetUrl = `${baseUrl}/api/v1/agent/activity/stream`;

  try {
    const upstream = await fetch(targetUrl, {
      headers: { Accept: "text/event-stream" },
      signal: request.signal,
    });

    if (!upstream.ok || !upstream.body) {
      return NextResponse.json(
        { error: "Failed to connect to agent activity stream" },
        { status: 502 },
      );
    }

    // Pipe the upstream SSE stream through to the client.
    const stream = new ReadableStream({
      async start(controller) {
        const reader = upstream.body!.getReader();
        const encoder = new TextEncoder();
        try {
          while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            controller.enqueue(value ?? encoder.encode(""));
          }
          controller.close();
        } catch {
          controller.close();
        }
      },
    });

    return new Response(stream, {
      headers: {
        "Content-Type": "text/event-stream",
        "Cache-Control": "no-cache",
        Connection: "keep-alive",
      },
    });
  } catch {
    return NextResponse.json(
      { error: "Failed to connect to agent" },
      { status: 502 },
    );
  }
}
