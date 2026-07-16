const VERSION = "0.1.0";
const RELEASE_BASE = `https://github.com/merdandt/LLM-wiki-dev/releases/download/v${VERSION}`;
const ARCHIVES = new Set([
  "llm-wiki-darwin-arm64.tar.gz",
  "llm-wiki-darwin-amd64.tar.gz",
  "llm-wiki-linux-arm64.tar.gz",
  "llm-wiki-linux-amd64.tar.gz",
  "llm-wiki-windows-amd64.tar.gz",
]);

const ASSET_PATHS = new Set([
  "/install.sh",
  "/releases/latest/release-manifest.json",
  `/releases/${VERSION}/release-manifest.json`,
]);

function assetResponse(response, path) {
  if (!response.ok) return new Response("Not Found\n", { status: 404 });
  const headers = new Headers(response.headers);
  headers.set(
    "content-type",
    path === "/install.sh" ? "text/plain; charset=utf-8" : "application/json; charset=utf-8",
  );
  headers.set("cache-control", path === "/install.sh" ? "public, max-age=300" : "public, max-age=60");
  return new Response(response.body, { status: response.status, headers });
}

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    if (request.method !== "GET" && request.method !== "HEAD") {
      return new Response("Method Not Allowed\n", { status: 405, headers: { allow: "GET, HEAD" } });
    }

    if (ASSET_PATHS.has(url.pathname)) {
      return assetResponse(await env.ASSETS.fetch(request), url.pathname);
    }

    const prefix = `/releases/${VERSION}/`;
    if (url.pathname.startsWith(prefix)) {
      const archive = url.pathname.slice(prefix.length);
      if (ARCHIVES.has(archive)) return Response.redirect(`${RELEASE_BASE}/${archive}`, 302);
    }

    return new Response("Not Found\n", { status: 404 });
  },
};
