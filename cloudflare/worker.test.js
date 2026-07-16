import test from "node:test";
import assert from "node:assert/strict";
import worker from "./worker.mjs";

const version = "0.1.0";
const releaseBase = `https://github.com/merdandt/LLM-wiki-dev/releases/download/v${version}`;

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

test("serves a version-pinned manifest and redirects immutable archives", async () => {
  const manifest = await worker.fetch(
    new Request(`https://example.test/releases/${version}/release-manifest.json`),
    { ASSETS: assets() },
  );
  assert.equal(manifest.status, 200);

  const archive = "llm-wiki-linux-amd64.tar.gz";
  const response = await worker.fetch(
    new Request(`https://example.test/releases/${version}/${archive}`),
    assets(),
  );
  assert.equal(response.status, 302);
  assert.equal(response.headers.get("location"), `${releaseBase}/${archive}`);
});

test("returns 404 for unknown paths and archive names", async () => {
  const unknown = await worker.fetch(new Request("https://example.test/nope"), { ASSETS: assets() });
  assert.equal(unknown.status, 404);

  const unknownArchive = await worker.fetch(
    new Request(`https://example.test/releases/${version}/unknown.tar.gz`),
    { ASSETS: assets() },
  );
  assert.equal(unknownArchive.status, 404);
});
