import type { NextRequest } from "next/server";
export const runtime = "nodejs";
export const dynamic = "force-dynamic";
const allowed = new Set([
  "health",
  "zones",
  "recommendations",
  "weather",
  "refresh",
  "deliveries",
  "analytics",
  "maps/venues",
  "maps/collect",
  "maps/job",
]);
async function proxy(
  request: NextRequest,
  context: { params: Promise<{ path: string[] }> },
) {
  const { path } = await context.params;
  const endpoint = path.join("/");
  if (!allowed.has(endpoint))
    return Response.json({ error: "Unknown API endpoint" }, { status: 404 });
  const backend = process.env.GO_API_URL || "http://127.0.0.1:8080";
  const url = new URL("/api/" + endpoint, backend);
  url.search = request.nextUrl.search;
  try {
    const response = await fetch(url, {
      method: request.method,
      headers: { "Content-Type": "application/json" },
      body: request.method === "POST" ? await request.text() : undefined,
      cache: "no-store",
      signal: AbortSignal.any([request.signal, AbortSignal.timeout(145000)]),
    });
    return new Response(response.body, {
      status: response.status,
      headers: {
        "Content-Type":
          response.headers.get("content-type") || "application/json",
        "Cache-Control": "no-store",
      },
    });
  } catch {
    return Response.json(
      {
        error:
          "Go backend unavailable. Start it with make run and check GO_API_URL.",
      },
      { status: 502 },
    );
  }
}
export { proxy as GET, proxy as POST, proxy as DELETE };
