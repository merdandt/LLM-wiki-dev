const RELEASE_ORIGIN = "https://github.com/merdandt/LLM-wiki-dev/releases/download";
const ARCHIVES = new Set([
  "llm-wiki-darwin-arm64.tar.gz",
  "llm-wiki-darwin-amd64.tar.gz",
  "llm-wiki-linux-arm64.tar.gz",
  "llm-wiki-linux-amd64.tar.gz",
  "llm-wiki-windows-amd64.tar.gz",
]);

const MANIFEST_PATH = /^\/releases\/(?:latest|\d+\.\d+\.\d+)\/release-manifest\.json$/;
const ARCHIVE_PATH = /^\/releases\/(\d+\.\d+\.\d+)\/([^/]+)$/;

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

    if (url.pathname === "/install.sh" || MANIFEST_PATH.test(url.pathname)) {
      return assetResponse(await env.ASSETS.fetch(request), url.pathname);
    }

    const archive = url.pathname.match(ARCHIVE_PATH);
    if (archive && ARCHIVES.has(archive[2])) {
      return Response.redirect(`${RELEASE_ORIGIN}/v${archive[1]}/${archive[2]}`, 302);
    }

    return new Response("Not Found\n", { status: 404 });
  },
};
