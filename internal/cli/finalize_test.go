package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setInitialized(t *testing.T, root string, value string) {
	t.Helper()
	path := filepath.Join(root, "llm-wiki.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(data), "initialized: "+map[string]string{"false": "true", "true": "false"}[value], "initialized: "+value, 1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFinalizeInitGatesOnStrictValidation(t *testing.T) {
	// A scaffold produced by the real template still carries `uninitialized`
	// fingerprints, so finalize-init must refuse with exit 4.
	root := hookFixture(t)
	template, err := filepath.Abs(filepath.Join("..", "..", "template"))
	if err != nil {
		t.Fatal(err)
	}
	scaffold := t.TempDir()
	runGitCmd(t, scaffold, "init", "-q")
	runGitCmd(t, scaffold, "-c", "user.name=T", "-c", "user.email=t@e.c", "commit", "-q", "--allow-empty", "-m", "root")
	var out, errBuf bytes.Buffer
	if code := Run([]string{"init", "--root", scaffold, "--template", template}, &out, &errBuf); code != 0 {
		t.Fatalf("init: code=%d stderr=%s", code, errBuf.String())
	}
	out.Reset()
	errBuf.Reset()
	if code := Run([]string{"finalize-init", "--root", scaffold}, &out, &errBuf); code != 4 {
		t.Fatalf("finalize-init on scaffold: code=%d, want 4; stderr=%s", code, errBuf.String())
	}

	// A compiled wiki (the valid fixture) with initialized:false finalizes.
	setInitialized(t, root, "false")
	out.Reset()
	errBuf.Reset()
	if code := Run([]string{"finalize-init", "--root", root}, &out, &errBuf); code != 0 {
		t.Fatalf("finalize-init on compiled wiki: code=%d stderr=%s", code, errBuf.String())
	}
	data, err := os.ReadFile(filepath.Join(root, "llm-wiki.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "initialized: true") {
		t.Fatalf("llm-wiki.yaml not flipped:\n%s", data)
	}
	// Idempotent rerun.
	out.Reset()
	errBuf.Reset()
	if code := Run([]string{"finalize-init", "--root", root}, &out, &errBuf); code != 0 || errBuf.Len() != 0 {
		t.Fatalf("rerun: code=%d stderr=%q", code, errBuf.String())
	}
}

func runGitCmd(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if outp, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, outp)
	}
}
