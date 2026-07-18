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

func TestRunInitWiresHookConfigs(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# fixture\n"), 0o644); err != nil {
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

	templateDir, err := filepath.Abs(filepath.Join("..", "..", "template"))
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"init", "--root", root, "--template", templateDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	for _, path := range []string{
		filepath.Join(root, ".claude", "settings.json"),
		filepath.Join(root, ".codex", "hooks.json"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatalf("%s: not valid JSON: %v", path, err)
		}
		hooks, _ := doc["hooks"].(map[string]any)
		if hooks == nil {
			t.Fatalf("%s: missing hooks key: %v", path, doc)
		}
		for _, event := range []string{"SessionStart", "Stop"} {
			entries, _ := hooks[event].([]any)
			if len(entries) == 0 {
				t.Fatalf("%s: missing %s entries: %v", path, event, doc)
			}
		}
		if !strings.Contains(string(data), ".llm-wiki/llm-wiki") || !strings.Contains(string(data), "hook") {
			t.Fatalf("%s: file missing llm-wiki hook command: %s", path, data)
		}
	}
}

func TestRunInitWritesMakefileTargets(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# fixture\n"), 0o644); err != nil {
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
	template, err := filepath.Abs(filepath.Join("..", "..", "template"))
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"init", "--root", root, "--template", template}, &stdout, &stderr); code != 0 {
		t.Fatalf("init: code=%d stderr=%s", code, stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "wiki-status:") {
		t.Fatalf("Makefile missing wiki targets:\n%s", data)
	}
}
