# Maintaining LLM Wiki

The canonical template lives in `template/`. Release archives are immutable GitHub Release artifacts. Cloudflare serves only the bootstrap installer and release metadata at `llm-wiki-dev.salesshortcut.ai`.

Publish a release by building each supported target with `scripts/package-release.sh`, recording the exact archive digests in the release manifest, uploading the archives to GitHub Releases, and publishing the manifest and installer through the subdomain.

Keep repository changes team-wide: commit the generated wiki and managed instruction blocks so Codex and Claude Code see the same memory. Update a page only from repository evidence, approved decisions, or confirmed reusable failure knowledge.

Before publishing:

- Run `make verify`.
- Run `sh -n release/install.sh scripts/package-release.sh`.
- Test the installer with a disposable Git repository.
- Confirm checksums and URLs are HTTPS.
- Confirm no transcripts, secrets, `.dart_tool`, `mason.yaml`, or mutable `main` URL appear in the release.
