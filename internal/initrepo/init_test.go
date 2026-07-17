package initrepo

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitializeIsIdempotentAndPreservesInstructions(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	agents := "# Existing team rules\n"
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(agents), 0o644); err != nil {
		t.Fatal(err)
	}
	template := filepath.Join("..", "..", "template")
	if err := Initialize(root, template); err != nil {
		t.Fatal(err)
	}
	first := snapshot(t, root)
	if err := Initialize(root, template); err != nil {
		t.Fatal(err)
	}
	second := snapshot(t, root)
	if first != second {
		t.Fatalf("second initialization changed the repository\nfirst=%s second=%s", first, second)
	}
	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), agents) || !strings.Contains(string(data), startMarker) {
		t.Fatalf("existing instruction text was not preserved: %s", data)
	}
	if _, err := os.Stat(filepath.Join(root, "mason.yaml")); !os.IsNotExist(err) {
		t.Fatal("unexpected mason.yaml")
	}
}

func TestInitializeIgnoresHelperDirectory(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	template := filepath.Join("..", "..", "template")
	if err := Initialize(root, template); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{".llm-wiki/", ".llm-wiki-state/"} {
		if !containsLine(string(data), line) {
			t.Fatalf(".gitignore is missing %q:\n%s", line, data)
		}
	}
}

func TestMergeGitignoreAddsOnlyMissingLines(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".gitignore")
	if err := os.WriteFile(path, []byte("node_modules/\n.llm-wiki-state/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := mergeGitignore(root, ".llm-wiki-state/\n.llm-wiki/\n"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if got := strings.Count(text, ".llm-wiki-state/"); got != 1 {
		t.Fatalf("expected exactly one .llm-wiki-state/ line, got %d:\n%s", got, text)
	}
	if !containsLine(text, ".llm-wiki/") {
		t.Fatalf(".gitignore is missing .llm-wiki/:\n%s", text)
	}
	if !containsLine(text, "node_modules/") {
		t.Fatalf("existing entries were lost:\n%s", text)
	}
}

func containsLine(text, want string) bool {
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, output)
	}
}

func snapshot(t *testing.T, root string) string {
	t.Helper()
	hash := sha256.New()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path != root && filepath.Base(path) == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, _ = hash.Write([]byte(filepath.ToSlash(strings.TrimPrefix(path, root))))
		_, _ = hash.Write(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}
