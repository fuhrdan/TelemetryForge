import { NextRequest } from "next/server";

export const dynamic = "force-dynamic";

const hopByHopHeaders = new Set([
  "connection",
  "keep-alive",
  "proxy-authenticate",
  "proxy-authorization",
  "te",
  "trailers",
  "transfer-encoding",
  "upgrade",
]);

function upstreamURL(request: NextRequest, path: string[]) {
  const base = process.env.TELEMETRYFORGE_API_BASE ?? "http://localhost:8080";
  const target = new URL(`/${path.join("/")}`, base);
  target.search = request.nextUrl.search;
  return target;
}

function upstreamHeaders(request: NextRequest) {
  const headers = new Headers();
  for (const [key, value] of request.headers.entries()) {
    const lower = key.toLowerCase();
    if (hopByHopHeaders.has(lower) || lower === "host" || lower === "authorization") {
      continue;
    }
    headers.set(key, value);
  }

  const key = process.env.TELEMETRYFORGE_DASHBOARD_API_KEY?.trim();
  if (key) {
    headers.set("Authorization", `Bearer ${key}`);
  }
  return headers;
}

function responseHeaders(upstream: Headers) {
  const headers = new Headers();
  for (const [key, value] of upstream.entries()) {
    if (!hopByHopHeaders.has(key.toLowerCase())) {
      headers.set(key, value);
    }
  }
  headers.set("X-Content-Type-Options", "nosniff");
  headers.set("Referrer-Policy", "no-referrer");
  return headers;
}

async function proxyRequest(
  request: NextRequest,
  path: string[],
  includeBody: boolean,
) {
  const response = await fetch(upstreamURL(request, path), {
    method: request.method,
    headers: upstreamHeaders(request),
    body: includeBody ? await request.arrayBuffer() : undefined,
    cache: "no-store",
    redirect: "manual",
  });

  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers: responseHeaders(response.headers),
  });
}

export async function GET(
  request: NextRequest,
  context: { params: Promise<{ path: string[] }> },
) {
  const { path } = await context.params;
  return proxyRequest(request, path, false);
}

export async function POST(
  request: NextRequest,
  context: { params: Promise<{ path: string[] }> },
) {
  const { path } = await context.params;
  return proxyRequest(request, path, true);
}
