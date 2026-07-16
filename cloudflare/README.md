# Cloudflare delivery layer

This Worker serves the public bootstrap installer and release manifests for
`llm-wiki-dev.salesshortcut.ai`. GitHub Releases remains the immutable origin
for the five platform archives; the Worker redirects versioned archive paths
to the matching GitHub Release asset.

The `site/` directory is the deployable static asset bundle. Keep it free of
credentials and generated binaries. Update the pinned version in
`worker.mjs`, the manifests, and the release assets together.

The production Worker and custom hostname must be managed through the
configured Cloudflare MCP server. Do not commit API tokens or account IDs.
