package hook

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/merdandt/LLM-wiki-dev/internal/config"
	"github.com/merdandt/LLM-wiki-dev/internal/gitrepo"
	"github.com/merdandt/LLM-wiki-dev/internal/state"
)

func buildFor(t *testing.T, root string) state.Fingerprint {
	t.Helper()
	repo, err := gitrepo.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(filepath.Join(root, "llm-wiki.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	fp, err := BuildFingerprint(repo, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return fp
}

func TestBuildFingerprintPartitions(t *testing.T) {
	root := initializedRepoFixture(t)
	base := buildFor(t, root)

	// Source change moves Evidence only.
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main // v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after := buildFor(t, root)
	if after.Evidence == base.Evidence {
		t.Fatal("source change did not move the evidence fingerprint")
	}
	if after.Wiki != base.Wiki {
		t.Fatal("source change moved the wiki fingerprint")
	}

	// Wiki change moves Wiki only.
	if err := os.WriteFile(filepath.Join(root, "docs", "llm-wiki", "glossary.md"),
		[]byte("# Glossary\n\nUpdated.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	final := buildFor(t, root)
	if final.Wiki == after.Wiki {
		t.Fatal("wiki change did not move the wiki fingerprint")
	}
	if final.Evidence != after.Evidence {
		t.Fatal("wiki change moved the evidence fingerprint")
	}

	// State writes move neither.
	if err := os.MkdirAll(filepath.Join(root, ".llm-wiki-state"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".llm-wiki-state", "scratch.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	unchanged := buildFor(t, root)
	if unchanged != final {
		t.Fatal("state write changed the fingerprint")
	}
}
