import { type NextRequest, NextResponse } from "next/server";
import { CHAT_TIMEOUT_MS } from "@/lib/constants";
import {
  jsonError,
  mapUpstreamError,
  readRuntimeMessageReply,
  runtimeURL,
  UpstreamError,
  waitForRuntimeSession,
} from "@/lib/quark-api";

export const dynamic = "force-dynamic";

export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ agentId: string }> },
) {
  await params;
  const baseUrl = request.nextUrl.searchParams.get("baseUrl");
  if (!baseUrl) return jsonError("baseUrl query param required", 400);

  try {
    const payload = await messagePayload(request);
    if (!payload.sessionKey) return jsonError("session_key required", 400);

    await waitForRuntimeSession(baseUrl, payload.sessionKey);

    const res = await fetch(
      runtimeURL(
        baseUrl,
        `/v1/sessions/${encodeURIComponent(payload.sessionKey)}/messages`,
      ),
      {
        method: "POST",
        headers: {
          Accept: "text/event-stream",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ content: payload.message }),
        signal: AbortSignal.timeout(CHAT_TIMEOUT_MS),
      },
    );

    if (!res.ok) {
      const data = await res.json().catch(() => ({ error: res.statusText }));
      return NextResponse.json(data, { status: res.status });
    }

    const { reply } = await readRuntimeMessageReply(res);
    return NextResponse.json({ reply });
  } catch (error) {
    return mapUpstreamError(error, "Runtime unreachable");
  }
}

async function messagePayload(request: NextRequest) {
  const contentType = request.headers.get("content-type") || "";
  if (contentType.includes("multipart/form-data")) {
    const form = await request.formData();
    const message = String(form.get("message") || "");
    const sessionKey = form.get("session_key");
    if (form.getAll("files").length > 0) {
      throw new UpstreamError(
        400,
        "File attachments are not supported by the current runtime API",
      );
    }
    return {
      message,
      sessionKey: typeof sessionKey === "string" ? sessionKey : undefined,
    };
  }

  const body = (await request.json()) as {
    message?: string;
    content?: string;
    session_key?: string;
    sessionKey?: string;
  };
  return {
    message: body.message ?? body.content ?? "",
    sessionKey: body.session_key ?? body.sessionKey,
  };
}
