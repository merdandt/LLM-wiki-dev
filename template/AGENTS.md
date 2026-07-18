<!-- llm-wiki:start -->
## LLM Wiki

Keep `{{wiki_path}}` as the team-shared, evidence-backed memory for this repository. Read `{{wiki_path}}/index.md` before exploring code and follow links to only the relevant pages. Hooks are quiet and local-only; they never call a network service during normal agent sessions.

If the Stop hook requests a maintenance pass: update the wiki pages affected by your diff, refresh README sections the changes invalidate (features, usage, install steps, API), run `.llm-wiki/llm-wiki validate`, then finish with `.llm-wiki/llm-wiki receipt write --kind synced` (or `--kind no-update --reason "<why>"` if nothing durable changed).
<!-- llm-wiki:end -->
