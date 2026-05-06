import { type NextRequest } from "next/server";
import {
  jsonError,
  runtimeURL,
  streamMessagesAsActivities,
} from "@/lib/quark-api";

export const dynamic = "force-dynamic";

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ sessionKey: string }> },
) {
  const { sessionKey } = await params;
  const baseUrl = request.nextUrl.searchParams.get("baseUrl");
  if (!baseUrl) return jsonError("baseUrl required", 400);

  try {
    const upstream = await fetch(
      runtimeURL(
        baseUrl,
        `/v1/sessions/${encodeURIComponent(sessionKey)}/messages/stream`,
      ),
      {
        headers: { Accept: "text/event-stream" },
        signal: request.signal,
      },
    );

    if (!upstream.ok || !upstream.body) {
      return jsonError("Failed to connect to runtime message stream", 502);
    }

    return new Response(streamMessagesAsActivities(upstream.body, sessionKey), {
      headers: {
        "Content-Type": "text/event-stream",
        "Cache-Control": "no-cache",
        Connection: "keep-alive",
      },
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : "Runtime unreachable";
    return jsonError(message, 502);
  }
}
