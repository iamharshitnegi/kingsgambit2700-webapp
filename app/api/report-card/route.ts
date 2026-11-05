import { NextResponse } from "next/server";

function retired() {
  return NextResponse.json({ type: "about:blank", title: "This endpoint has been retired", status: 410, detail: "Report generation is not part of the academy platform.", code: "endpoint_retired" }, { status: 410, headers: { "Cache-Control": "no-store", "Content-Type": "application/problem+json" } });
}

export const GET = retired;
export const POST = retired;
