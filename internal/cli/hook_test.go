package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func hookFixture(t *testing.T) string {
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
	for _, args := range [][]string{
		{"init", "-q"}, {"add", "."},
		{"-c", "user.name=T", "-c", "user.email=t@e.c", "commit", "-qm", "baseline"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func runHookCommand(t *testing.T, root, event, sessionID string, extra map[string]any) (string, string, int) {
	t.Helper()
	payload := map[string]any{"session_id": sessionID, "cwd": root,
		"hook_event_name": map[string]string{"session-start": "SessionStart", "stop": "Stop"}[event]}
	for k, v := range extra {
		payload[k] = v
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := RunWithStdin(bytes.NewReader(data), []string{"hook", event}, &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func TestHookLifecycleEndToEnd(t *testing.T) {
	root := hookFixture(t)

	// 1. Session start prints the packet.
	stdout, stderr, code := runHookCommand(t, root, "session-start", "s1", nil)
	if code != 0 || stderr != "" || !strings.Contains(stdout, "team memory") {
		t.Fatalf("session-start: code=%d stderr=%q stdout=%q", code, stderr, stdout)
	}

	// 2. Stop with no changes is byte-silent.
	stdout, stderr, code = runHookCommand(t, root, "stop", "s1", nil)
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("clean stop: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	// 3. Material change drifts with block JSON.
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main // v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, code = runHookCommand(t, root, "stop", "s1", nil)
	if code != 0 {
		t.Fatalf("drift stop exit = %d", code)
	}
	var block map[string]string
	if err := json.Unmarshal([]byte(stdout), &block); err != nil {
		t.Fatalf("drift stdout not JSON: %q", stdout)
	}
	if block["decision"] != "block" || block["reason"] == "" {
		t.Fatalf("unexpected block payload: %v", block)
	}

	// 4. receipt write closes the loop (agent pass simulated by a wiki edit).
	// Heading must match the log validator's format: "## [YYYY-MM-DD] (init|sync|audit|migrate) | ...".
	logPath := filepath.Join(root, "docs", "llm-wiki", "log.md")
	if err := os.WriteFile(logPath,
		appendLine(t, logPath, "\n## [2026-07-18] sync | Updated for main.go change.\n\nUpdated for main.go change.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	code = Run([]string{"receipt", "write", "--kind", "synced", "--root", root}, &stdoutBuf, &stderrBuf)
	if code != 0 || stdoutBuf.Len() != 0 || stderrBuf.Len() != 0 {
		t.Fatalf("receipt write: code=%d stdout=%q stderr=%q", code, stdoutBuf.String(), stderrBuf.String())
	}

	// 5. Next stop is silent again.
	stdout, stderr, code = runHookCommand(t, root, "stop", "s1", nil)
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("post-receipt stop: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func appendLine(t *testing.T, path, line string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return append(data, []byte(line)...)
}

func TestReceiptWriteWithoutLease(t *testing.T) {
	root := hookFixture(t)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"receipt", "write", "--kind", "synced", "--root", root}, &stdout, &stderr)
	if code != 5 {
		t.Fatalf("exit = %d, want 5 (no active lease); stderr=%q", code, stderr.String())
	}
}

func TestReceiptWriteCorruptSessionSurfacesError(t *testing.T) {
	root := hookFixture(t)

	if _, _, code := runHookCommand(t, root, "session-start", "s1", nil); code != 0 {
		t.Fatalf("session-start code=%d", code)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main // v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, code := runHookCommand(t, root, "stop", "s1", nil)
	if code != 0 {
		t.Fatalf("drift stop code=%d", code)
	}
	var block map[string]string
	if err := json.Unmarshal([]byte(stdout), &block); err != nil || block["decision"] != "block" {
		t.Fatalf("expected drift block (lease held), got stdout=%q", stdout)
	}

	// Corrupt the session file for the current lease owner (s1).
	sum := sha256.Sum256([]byte("s1"))
	sessionPath := filepath.Join(root, ".llm-wiki-state", "sessions", hex.EncodeToString(sum[:])+".json")
	if err := os.WriteFile(sessionPath, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	code = Run([]string{"receipt", "write", "--kind", "synced", "--root", root}, &stdoutBuf, &stderrBuf)
	if code != 3 {
		t.Fatalf("exit = %d, want 3 (corrupted session); stdout=%q stderr=%q", code, stdoutBuf.String(), stderrBuf.String())
	}
	if stderrBuf.Len() == 0 {
		t.Fatalf("expected a stderr message for the corrupted session")
	}
}

func TestHookNeverBreaksSession(t *testing.T) {
	// Garbage stdin: one stderr line, exit 0.
	var stdout, stderr bytes.Buffer
	code := RunWithStdin(strings.NewReader("not json"), []string{"hook", "stop"}, &stdout, &stderr)
	if code != 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "llm-wiki") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestHookEventSubcommandMismatch(t *testing.T) {
	root := t.TempDir()
	payload := map[string]any{"session_id": "s1", "cwd": root, "hook_event_name": "SessionStart"}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := RunWithStdin(bytes.NewReader(data), []string{"hook", "stop"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	lines := strings.Split(strings.TrimRight(stderr.String(), "\n"), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "does not match") {
		t.Fatalf("stderr = %q, want exactly one line containing %q", stderr.String(), "does not match")
	}
}

func TestReceiptWriteUsageErrors(t *testing.T) {
	root := hookFixture(t)
	longReason := strings.Repeat("a", 501)
	cases := []struct {
		name string
		args []string
	}{
		{"unknown kind", []string{"receipt", "write", "--kind", "bogus", "--root", root}},
		{"no-update without reason", []string{"receipt", "write", "--kind", "no-update", "--root", root}},
		{"no-update reason too long", []string{"receipt", "write", "--kind", "no-update", "--reason", longReason, "--root", root}},
		{"unknown hook subcommand", []string{"hook", "bogus"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tc.args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("exit = %d, want 2; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestReceiptWriteRootNotGitRepo(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"receipt", "write", "--kind", "synced", "--root", root}, &stdout, &stderr)
	if code != 3 {
		t.Fatalf("exit = %d, want 3 (not a git repository); stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stderr.Len() == 0 {
		t.Fatalf("expected a stderr message")
	}
}

func TestReceiptWriteValidationGate(t *testing.T) {
	root := hookFixture(t)

	if _, _, code := runHookCommand(t, root, "session-start", "s1", nil); code != 0 {
		t.Fatalf("session-start code=%d", code)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main // v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, code := runHookCommand(t, root, "stop", "s1", nil)
	if code != 0 {
		t.Fatalf("drift stop code=%d", code)
	}
	var block map[string]string
	if err := json.Unmarshal([]byte(stdout), &block); err != nil || block["decision"] != "block" {
		t.Fatalf("expected drift block (lease held), got stdout=%q", stdout)
	}

	// Break wiki validation deterministically: a page with a relative link to
	// a page that does not exist trips the validator's "broken-link" check.
	brokenPath := filepath.Join(root, "docs", "llm-wiki", "broken.md")
	brokenContent := "---\n" +
		"id: broken.page\n" +
		"kind: component\n" +
		"status: current\n" +
		"summary: Temporary page with a broken link for validation-gate coverage.\n" +
		"verification:\n" +
		"  base_commit: abc\n" +
		"  evidence_fingerprint: sha256:0000000000000000000000000000000000000000000000000000000000000000\n" +
		"evidence: []\n" +
		"relations: []\n" +
		"---\n" +
		"# Broken\n\n" +
		"See [Nonexistent](nonexistent-page.md) for details.\n"
	if err := os.WriteFile(brokenPath, []byte(brokenContent), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	code = Run([]string{"receipt", "write", "--kind", "synced", "--root", root}, &stdoutBuf, &stderrBuf)
	if code != 4 {
		t.Fatalf("exit = %d, want 4 (validation gate); stdout=%q stderr=%q", code, stdoutBuf.String(), stderrBuf.String())
	}

	if err := os.Remove(brokenPath); err != nil {
		t.Fatal(err)
	}
	stdoutBuf.Reset()
	stderrBuf.Reset()
	code = Run([]string{"receipt", "write", "--kind", "synced", "--root", root}, &stdoutBuf, &stderrBuf)
	if code != 0 || stdoutBuf.Len() != 0 || stderrBuf.Len() != 0 {
		t.Fatalf("receipt write after fix: code=%d stdout=%q stderr=%q", code, stdoutBuf.String(), stderrBuf.String())
	}
}
