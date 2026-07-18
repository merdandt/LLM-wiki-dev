package initrepo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readHooks(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func hookEntries(t *testing.T, doc map[string]any, event string) []any {
	t.Helper()
	hooks, _ := doc["hooks"].(map[string]any)
	entries, _ := hooks[event].([]any)
	return entries
}

func TestWriteHookConfigsFresh(t *testing.T) {
	root := t.TempDir()
	warnings, err := WriteHookConfigs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	for _, path := range []string{
		filepath.Join(root, ".claude", "settings.json"),
		filepath.Join(root, ".codex", "hooks.json"),
	} {
		doc := readHooks(t, path)
		if len(hookEntries(t, doc, "SessionStart")) != 1 || len(hookEntries(t, doc, "Stop")) != 1 {
			t.Fatalf("%s: missing hook entries: %v", path, doc)
		}
	}
}

func TestWriteHookConfigsIdempotent(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 2; i++ {
		if _, err := WriteHookConfigs(root); err != nil {
			t.Fatal(err)
		}
	}
	doc := readHooks(t, filepath.Join(root, ".claude", "settings.json"))
	if got := len(hookEntries(t, doc, "Stop")); got != 1 {
		t.Fatalf("Stop entries = %d, want 1 after rerun", got)
	}
}

func TestWriteHookConfigsPreservesForeignContent(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{
  "permissions": {"allow": ["Bash(go test:*)"]},
  "hooks": {"Stop": [{"hooks": [{"type": "command", "command": "echo done"}]}]}
}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteHookConfigs(root); err != nil {
		t.Fatal(err)
	}
	doc := readHooks(t, filepath.Join(claudeDir, "settings.json"))
	if _, ok := doc["permissions"]; !ok {
		t.Fatal("foreign permissions key was dropped")
	}
	if got := len(hookEntries(t, doc, "Stop")); got != 2 {
		t.Fatalf("Stop entries = %d, want foreign + ours", got)
	}
}

func TestWriteHookConfigsSkipsMalformed(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	warnings, err := WriteHookConfigs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one about the malformed file", warnings)
	}
	data, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{broken" {
		t.Fatal("malformed file was modified")
	}
}

func TestWriteHookConfigsEmptyFile(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	codexDir := filepath.Join(root, ".codex")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	claudePath := filepath.Join(claudeDir, "settings.json")
	codexPath := filepath.Join(codexDir, "hooks.json")
	if err := os.WriteFile(claudePath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codexPath, []byte("  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	warnings, err := WriteHookConfigs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	for _, path := range []string{claudePath, codexPath} {
		doc := readHooks(t, path)
		if len(hookEntries(t, doc, "SessionStart")) != 1 || len(hookEntries(t, doc, "Stop")) != 1 {
			t.Fatalf("%s: missing hook entries: %v", path, doc)
		}
	}
}

func TestWriteHookConfigsSkipsUnexpectedHooksShape(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"hooks": "corrupt"}`)
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	warnings, err := WriteHookConfigs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one about the unexpected hooks shape", warnings)
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(original) {
		t.Fatalf("file was modified: got %s, want %s", data, original)
	}
}

func TestWriteHookConfigsSkipsUnexpectedEventShape(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"hooks": {"Stop": "not-an-array"}}`)
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	warnings, err := WriteHookConfigs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one about the unexpected hooks shape", warnings)
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(original) {
		t.Fatalf("file was modified: got %s, want %s", data, original)
	}
}
