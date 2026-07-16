package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	got := Default()
	if got.SchemaVersion != 1 || got.WikiPath != "docs/llm-wiki" {
		t.Fatalf("unexpected defaults: %#v", got)
	}
	if got.ContextBudgetBytes != 12*1024 || got.IndexEntryLimit != 200 {
		t.Fatalf("unexpected budgets: %#v", got)
	}
	if got.LockWaitSeconds != 5 || got.SyncLeaseSeconds != 600 ||
		got.Maintenance.MaxRecoveryPasses != 1 {
		t.Fatalf("unexpected maintenance defaults: %#v", got)
	}
}

func TestLoadRejectsUnsafePaths(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "wiki traversal", body: "wiki_path: ../outside\nstate_path: .llm-wiki-state\n"},
		{name: "wiki git", body: "wiki_path: .git/hooks\nstate_path: .llm-wiki-state\n"},
		{name: "state git", body: "wiki_path: docs/llm-wiki\nstate_path: .git/llm-wiki\n"},
		{name: "same paths", body: "wiki_path: docs/llm-wiki\nstate_path: docs/llm-wiki\n"},
		{name: "nested paths", body: "wiki_path: docs\nstate_path: docs/llm-wiki-state\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "llm-wiki.yaml")
			body := "schema_version: 1\ninitialized: false\n" + tt.body
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("Load() error = nil, want path validation error")
			}
		})
	}
}

func TestLoadRejectsSymlinkedConfig(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.yaml")
	link := filepath.Join(root, "llm-wiki.yaml")
	if err := os.WriteFile(target, []byte("schema_version: 1\ninitialized: false\nwiki_path: docs/llm-wiki\nstate_path: .llm-wiki-state\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := Load(link); err == nil {
		t.Fatal("Load() error = nil, want symlink rejection")
	}
}
