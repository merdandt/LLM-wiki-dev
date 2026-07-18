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
	if code != 0 {
		t.Fatalf("receipt write: code=%d stderr=%q", code, stderrBuf.String())
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

func TestHookNeverBreaksSession(t *testing.T) {
	// Garbage stdin: one stderr line, exit 0.
	var stdout, stderr bytes.Buffer
	code := RunWithStdin(strings.NewReader("not json"), []string{"hook", "stop"}, &stdout, &stderr)
	if code != 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "llm-wiki") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
