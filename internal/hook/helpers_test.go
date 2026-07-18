package hook

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/merdandt/LLM-wiki-dev/internal/gitrepo"
)

func initializedRepoFixture(t *testing.T) string {
	t.Helper()
	source, err := filepath.Abs(filepath.Join("..", "..", "testdata", "wiki", "valid"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init", "-q")
	runGit(t, root, "add", ".")
	runGit(t, root,
		"-c", "user.name=Hook Test", "-c", "user.email=hook@example.com",
		"commit", "-qm", "baseline")
	// t.TempDir may sit behind a symlink on macOS; match gitrepo.Discover.
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func discoverRepo(t *testing.T, root string) gitrepo.Repo {
	t.Helper()
	repo, err := gitrepo.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func writeAndCommit(t *testing.T, root, relative, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root,
		"-c", "user.name=Hook Test", "-c", "user.email=hook@example.com",
		"commit", "-qm", "update")
}
