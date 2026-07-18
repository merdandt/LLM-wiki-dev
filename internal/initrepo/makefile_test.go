package initrepo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteMakefileTargetsCreates(t *testing.T) {
	root := t.TempDir()
	if err := WriteMakefileTargets(root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"# --- llm-wiki start ---",
		"# --- llm-wiki end ---",
		".PHONY: wiki-status wiki-validate wiki-finalize",
		"wiki-status:",
		"\t./.llm-wiki/llm-wiki status",
		"wiki-validate:",
		"\t./.llm-wiki/llm-wiki validate",
		"wiki-finalize:",
		"\t./.llm-wiki/llm-wiki finalize-init",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Makefile missing %q:\n%s", want, text)
		}
	}
}

func TestWriteMakefileTargetsAppendsAndPreserves(t *testing.T) {
	root := t.TempDir()
	existing := "build:\n\tgo build ./...\n"
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteMakefileTargets(root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.HasPrefix(text, existing) {
		t.Fatalf("existing content not preserved at top:\n%s", text)
	}
	if !strings.Contains(text, "wiki-status:") {
		t.Fatalf("targets not appended:\n%s", text)
	}
}

func TestWriteMakefileTargetsIdempotentAndUpgrades(t *testing.T) {
	root := t.TempDir()
	if err := WriteMakefileTargets(root); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteMakefileTargets(root); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("rerun changed the Makefile:\nfirst:\n%s\nsecond:\n%s", first, second)
	}

	// A stale block between markers is replaced, content outside untouched.
	stale := "deploy:\n\techo deploy\n\n# --- llm-wiki start ---\nold-target:\n\techo old\n# --- llm-wiki end ---\n"
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteMakefileTargets(root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "deploy:") {
		t.Fatalf("foreign target lost:\n%s", text)
	}
	if strings.Contains(text, "old-target") {
		t.Fatalf("stale block not replaced:\n%s", text)
	}
	if !strings.Contains(text, "wiki-status:") {
		t.Fatalf("current targets missing:\n%s", text)
	}
	if strings.Count(text, "# --- llm-wiki start ---") != 1 {
		t.Fatalf("marker duplicated:\n%s", text)
	}
}
