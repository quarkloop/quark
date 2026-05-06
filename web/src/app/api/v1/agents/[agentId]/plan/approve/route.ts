import { NextResponse } from "next/server";

export const dynamic = "force-dynamic";

export async function POST() {
  return NextResponse.json(
    { error: "Plan approval is not exposed by the current runtime API" },
    { status: 404 },
  );
}
