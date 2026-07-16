package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"version"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "llm-wiki dev" {
		t.Fatalf("stdout = %q, want %q", got, "llm-wiki dev")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunValidateValidAndInvalid(t *testing.T) {
	root := copyCLIFixture(t)
	initCLIGit(t, root)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"validate", "--root", root}, &stdout, &stderr); code != 0 {
		t.Fatalf("valid validate code=%d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var report struct {
		Errors []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Errors) != 0 {
		t.Fatalf("valid report errors=%v", report.Errors)
	}

	data, err := os.ReadFile(filepath.Join(root, "docs/llm-wiki/system.md"))
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte("system.root"), []byte("index.root"), 1)
	if err := os.WriteFile(filepath.Join(root, "docs/llm-wiki/duplicate.md"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"validate", "--root", root}, &stdout, &stderr); code != 4 {
		t.Fatalf("invalid validate code=%d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
}

func TestRunFingerprintJSONIsDeterministicAndRejectsUnsafeInputs(t *testing.T) {
	root := copyCLIFixture(t)
	initCLIGit(t, root)
	evidencePath := filepath.Join(root, "evidence.txt")
	if err := os.WriteFile(evidencePath, []byte("evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pagePath := filepath.Join(root, "docs/llm-wiki/system.md")
	page, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatal(err)
	}
	page = bytes.Replace(page, []byte("evidence: []"), []byte("evidence:\n  - path: evidence.txt"), 1)
	if err := os.WriteFile(pagePath, page, 0o644); err != nil {
		t.Fatal(err)
	}
	var firstOut, firstErr bytes.Buffer
	if code := Run([]string{"fingerprint", "--root", root, "--page", "docs/llm-wiki/system.md", "--json"}, &firstOut, &firstErr); code != 0 {
		t.Fatalf("fingerprint code=%d stderr=%q", code, firstErr.String())
	}
	var secondOut, secondErr bytes.Buffer
	if code := Run([]string{"fingerprint", "--root", root, "--page", "docs/llm-wiki/system.md", "--json"}, &secondOut, &secondErr); code != 0 {
		t.Fatalf("second fingerprint code=%d stderr=%q", code, secondErr.String())
	}
	if firstOut.String() != secondOut.String() {
		t.Fatalf("fingerprint output is not deterministic: %q != %q", firstOut.String(), secondOut.String())
	}
	var got struct {
		BaseCommit          string `json:"base_commit"`
		EvidenceFingerprint string `json:"evidence_fingerprint"`
	}
	if err := json.Unmarshal(firstOut.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.BaseCommit == "" || !strings.HasPrefix(got.EvidenceFingerprint, "sha256:") {
		t.Fatalf("unexpected fingerprint: %#v", got)
	}

	page = bytes.Replace(page, []byte("evidence.txt"), []byte("missing.txt"), 1)
	if err := os.WriteFile(pagePath, page, 0o644); err != nil {
		t.Fatal(err)
	}
	var unsafeOut, unsafeErr bytes.Buffer
	if code := Run([]string{"fingerprint", "--root", root, "--page", "../outside.md", "--json"}, &unsafeOut, &unsafeErr); code == 0 {
		t.Fatal("escaping page path unexpectedly succeeded")
	}
	unsafeOut.Reset()
	unsafeErr.Reset()
	if code := Run([]string{"fingerprint", "--root", root, "--page", "docs/llm-wiki/system.md", "--json"}, &unsafeOut, &unsafeErr); code == 0 {
		t.Fatal("missing evidence unexpectedly succeeded")
	}
}

func copyCLIFixture(t *testing.T) string {
	t.Helper()
	source := filepath.Join("..", "..", "testdata", "wiki", "valid")
	target := t.TempDir()
	if err := copyCLIDirectory(source, target); err != nil {
		t.Fatal(err)
	}
	return target
}

func copyCLIDirectory(source, target string) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(source, entry.Name())
		targetPath := filepath.Join(target, entry.Name())
		if entry.IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
			if err := copyCLIDirectory(sourcePath, targetPath); err != nil {
				return err
			}
			continue
		}
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func initCLIGit(t *testing.T, root string) {
	t.Helper()
	cmd := exec.Command("git", "-C", root, "init")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
}
