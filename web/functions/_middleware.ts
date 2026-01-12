const PROXY_PATH_PREFIXES = [
  "/.well-known/",
  "/api/",
  "/device/",
  "/github-app/",
]

const PROXY_EXACT_PATHS = ["/authorize", "/callback", "/token", "/refresh", "/logout", "/healthz"]

function shouldProxy(pathname: string): boolean {
  if (PROXY_EXACT_PATHS.includes(pathname)) return true
  return PROXY_PATH_PREFIXES.some((prefix) => pathname.startsWith(prefix))
}

export async function onRequest(context: any): Promise<Response> {
  const request: Request = context.request
  const url = new URL(request.url)

  if (!shouldProxy(url.pathname)) {
    return context.next()
  }

  const apiOrigin = (context.env && context.env.API_ORIGIN) || "https://api.rocketship.sh"
  const upstreamURL = new URL(url.pathname + url.search, apiOrigin)

  const upstreamRequest = new Request(upstreamURL.toString(), request)

  const response = await fetch(upstreamRequest, {
    redirect: "manual",
  })

  // Ensure auth-related endpoints are never cached by the edge.
  const headers = new Headers(response.headers)
  headers.set("Cache-Control", "no-store")

  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers,
  })
}

