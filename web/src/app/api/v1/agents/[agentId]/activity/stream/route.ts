import { type NextRequest } from "next/server";
import { jsonError } from "@/lib/quark-api";

export const dynamic = "force-dynamic";

export async function GET(request: NextRequest) {
  const sessionKey = request.nextUrl.searchParams.get("sessionKey");
  if (!sessionKey) {
    return jsonError("sessionKey required for runtime message streams", 400);
  }
  const target = new URL(request.url);
  target.pathname = `${target.pathname.replace(/\/activity\/stream$/, "")}/sessions/${encodeURIComponent(sessionKey)}/activity/stream`;
  target.searchParams.delete("sessionKey");
  return Response.redirect(target, 307);
}
