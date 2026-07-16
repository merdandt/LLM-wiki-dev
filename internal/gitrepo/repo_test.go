package gitrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverFromNestedDirectory(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	repo, err := Discover(nested)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if repo.Root != expected {
		t.Fatalf("Root = %q, want %q", repo.Root, expected)
	}
}

func TestChangedPathsAndPatchExcludeProjectMemory(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	writeFile(t, root, "internal/orders/service.go", "package orders\n")
	runGit(t, root, "add", "internal/orders/service.go")
	runGit(t, root, "commit", "-m", "baseline")
	repo, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	base, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "internal/orders/service.go", "package orders\n\n// changed\n")
	writeFile(t, root, "internal/newfeature/feature.go", "package newfeature\n")
	writeFile(t, root, "docs/llm-wiki/system.md", "wiki\n")
	writeFile(t, root, ".llm-wiki-state/session.json", "state\n")
	writeFile(t, root, "llm-wiki.yaml", "config\n")

	paths, err := repo.ChangedPaths(base)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(paths, "\n")
	for _, want := range []string{"internal/orders/service.go", "internal/newfeature/feature.go", "docs/llm-wiki/system.md"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("ChangedPaths missing %q: %v", want, paths)
		}
	}

	patch, err := repo.WorktreePatch("docs/llm-wiki", ".llm-wiki-state", "llm-wiki.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(patch, "internal/orders/service.go") {
		t.Fatalf("patch missing source change: %q", patch)
	}
	for _, excluded := range []string{"docs/llm-wiki/system.md", ".llm-wiki-state/session.json", "llm-wiki.yaml"} {
		if strings.Contains(patch, excluded) {
			t.Fatalf("patch contains excluded path %q: %q", excluded, patch)
		}
	}

	untracked, err := repo.UntrackedPaths("docs/llm-wiki", ".llm-wiki-state", "llm-wiki.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(untracked) != 1 || untracked[0] != "internal/newfeature/feature.go" {
		t.Fatalf("UntrackedPaths = %v", untracked)
	}
}

func TestUnbornRepositoryHeadAndChanges(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	writeFile(t, root, "internal/orders/service.go", "package orders\n")
	repo, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	if head != "4b825dc642cb6eb9a060e54bf8d69288fbee4904" {
		t.Fatalf("Head = %q, want empty tree", head)
	}
	paths, err := repo.ChangedPaths(head)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "internal/orders/service.go" {
		t.Fatalf("ChangedPaths = %v", paths)
	}
	patch, err := repo.WorktreePatch()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(patch, "internal/orders/service.go") {
		t.Fatalf("patch missing unborn-repo file: %q", patch)
	}
}

func writeFile(t *testing.T, root, relative, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=LLM Wiki Test",
		"GIT_AUTHOR_EMAIL=llm-wiki@example.test",
		"GIT_COMMITTER_NAME=LLM Wiki Test",
		"GIT_COMMITTER_EMAIL=llm-wiki@example.test",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}
