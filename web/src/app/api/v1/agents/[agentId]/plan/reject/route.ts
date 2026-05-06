import { NextResponse } from "next/server";

export const dynamic = "force-dynamic";

export async function POST() {
  return NextResponse.json(
    { error: "Plan rejection is not exposed by the current runtime API" },
    { status: 404 },
  );
}
