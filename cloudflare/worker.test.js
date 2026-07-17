import test from "node:test";
import assert from "node:assert/strict";
import worker from "./worker.mjs";

const releaseOrigin = "https://github.com/merdandt/LLM-wiki-dev/releases/download";

function assets() {
  return {
    fetch(request) {
      return new Response(`asset:${new URL(request.url).pathname}`, {
        headers: { "content-type": "text/plain" },
      });
    },
  };
}

test("serves the installer and latest manifest from static assets", async () => {
  const install = await worker.fetch(new Request("https://example.test/install.sh"), { ASSETS: assets() });
  assert.equal(install.status, 200);
  assert.equal(install.headers.get("content-type"), "text/plain; charset=utf-8");

  const manifest = await worker.fetch(
    new Request("https://example.test/releases/latest/release-manifest.json"),
    { ASSETS: assets() },
  );
  assert.equal(manifest.status, 200);
  assert.equal(manifest.headers.get("content-type"), "application/json; charset=utf-8");
});

test("serves manifests and redirects immutable archives for any release version", async () => {
  const archive = "llm-wiki-linux-amd64.tar.gz";
  for (const version of ["0.1.0", "0.1.1", "2.3.4"]) {
    const manifest = await worker.fetch(
      new Request(`https://example.test/releases/${version}/release-manifest.json`),
      { ASSETS: assets() },
    );
    assert.equal(manifest.status, 200, `manifest for ${version}`);
    assert.equal(manifest.headers.get("content-type"), "application/json; charset=utf-8");

    const response = await worker.fetch(
      new Request(`https://example.test/releases/${version}/${archive}`),
      { ASSETS: assets() },
    );
    assert.equal(response.status, 302, `redirect for ${version}`);
    assert.equal(response.headers.get("location"), `${releaseOrigin}/v${version}/${archive}`);
  }
});

test("returns 404 for unknown paths, versions, and archive names", async () => {
  for (const path of [
    "/nope",
    "/releases/0.1.0/unknown.tar.gz",
    "/releases/abc/llm-wiki-linux-amd64.tar.gz",
    "/releases/0.1/release-manifest.json",
    "/releases/0.1.0/../llm-wiki-linux-amd64.tar.gz",
  ]) {
    const response = await worker.fetch(new Request(`https://example.test${path}`), { ASSETS: assets() });
    assert.equal(response.status, 404, `expected 404 for ${path}`);
  }
});

test("rejects non-read methods", async () => {
  const response = await worker.fetch(
    new Request("https://example.test/install.sh", { method: "POST" }),
    { ASSETS: assets() },
  );
  assert.equal(response.status, 405);
  assert.equal(response.headers.get("allow"), "GET, HEAD");
});
