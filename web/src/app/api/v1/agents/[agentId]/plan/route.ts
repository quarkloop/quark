import { NextResponse } from "next/server";

export const dynamic = "force-dynamic";

export async function GET() {
  return NextResponse.json(
    { error: "Plan endpoints are not exposed by the current runtime API" },
    { status: 404 },
  );
}
