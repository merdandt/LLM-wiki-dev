package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestBuiltBinaryHookFlow drives the real binary the way Claude Code and
// Codex do: JSON on stdin, silence or block JSON on stdout.
func TestBuiltBinaryHookFlow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell guard commands are not exercised on Windows")
	}
	binary := filepath.Join(t.TempDir(), "llm-wiki")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/llm-wiki")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	root := hookFixture(t)

	run := func(event, name string) (string, string, int) {
		payload, err := json.Marshal(map[string]any{
			"session_id": "it1", "cwd": root, "hook_event_name": name,
		})
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(binary, "hook", event)
		cmd.Stdin = bytes.NewReader(payload)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err = cmd.Run()
		code := 0
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return stdout.String(), stderr.String(), code
	}

	stdout, stderr, code := run("session-start", "SessionStart")
	if code != 0 || !strings.Contains(stdout, "team memory") || stderr != "" {
		t.Fatalf("session-start: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	stdout, stderr, code = run("stop", "Stop")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("clean stop: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main // drift\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, code = run("stop", "Stop")
	if code != 0 || !strings.Contains(stdout, `"decision"`) {
		t.Fatalf("drift stop: code=%d stdout=%q", code, stdout)
	}

	// The guard command is a silent no-op without the binary.
	guard := exec.Command("sh", "-c",
		`[ -x "$CLAUDE_PROJECT_DIR/.llm-wiki/llm-wiki" ] && "$CLAUDE_PROJECT_DIR/.llm-wiki/llm-wiki" hook stop || exit 0`)
	guard.Env = append(os.Environ(), "CLAUDE_PROJECT_DIR="+root)
	var guardOut bytes.Buffer
	guard.Stdout = &guardOut
	if err := guard.Run(); err != nil || guardOut.Len() != 0 {
		t.Fatalf("guard without binary: err=%v out=%q", err, guardOut.String())
	}
}
