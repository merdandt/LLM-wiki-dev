package initrepo

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/merdandt/LLM-wiki-dev/internal/atomicfile"
)

// hookMarker matches the ".llm-wiki/llm-wiki" hook substring as it appears
// inside JSON-encoded command text, where json.Marshal escapes the literal
// quote as \" (backslash + quote).
const hookMarker = `.llm-wiki/llm-wiki\" hook`

func claudeCommand(event string) string {
	return `[ -x "$CLAUDE_PROJECT_DIR/.llm-wiki/llm-wiki" ] && "$CLAUDE_PROJECT_DIR/.llm-wiki/llm-wiki" hook ` + event + ` || exit 0`
}

func codexCommand(event string) string {
	return `root="$(git rev-parse --show-toplevel 2>/dev/null)" && [ -x "$root/.llm-wiki/llm-wiki" ] && "$root/.llm-wiki/llm-wiki" hook ` + event + ` || exit 0`
}

func hookEvents(command func(string) string) map[string][]any {
	sessionStart := map[string]any{
		"matcher": "startup|resume|clear|compact",
		"hooks": []any{map[string]any{
			"type": "command", "command": command("session-start"), "timeout": 10,
		}},
	}
	stop := map[string]any{
		"hooks": []any{map[string]any{
			"type": "command", "command": command("stop"), "timeout": 15,
		}},
	}
	return map[string][]any{"SessionStart": {sessionStart}, "Stop": {stop}}
}

// WriteHookConfigs writes or conservatively merges the LLM Wiki lifecycle
// hooks into .claude/settings.json and .codex/hooks.json. Foreign hooks and
// settings keys are never modified; malformed files are left untouched and
// reported as warnings.
func WriteHookConfigs(root string) ([]string, error) {
	var warnings []string
	targets := []struct {
		path    string
		command func(string) string
	}{
		{filepath.Join(root, ".claude", "settings.json"), claudeCommand},
		{filepath.Join(root, ".codex", "hooks.json"), codexCommand},
	}
	for _, target := range targets {
		warning, err := mergeHookFile(target.path, hookEvents(target.command))
		if err != nil {
			return warnings, err
		}
		if warning != "" {
			warnings = append(warnings, warning)
		}
	}
	return warnings, nil
}

func mergeHookFile(path string, events map[string][]any) (string, error) {
	doc := map[string]any{}
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return "", err
	default:
		if err := json.Unmarshal(data, &doc); err != nil {
			return fmt.Sprintf("llm-wiki: %s is not valid JSON; add the LLM Wiki hooks manually", path), nil
		}
	}
	hooks, _ := doc["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	changed := false
	for event, entries := range events {
		existing, _ := hooks[event].([]any)
		if containsMarker(existing) {
			continue
		}
		hooks[event] = append(existing, entries...)
		changed = true
	}
	if !changed {
		return "", nil
	}
	doc["hooks"] = hooks
	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return "", atomicfile.Write(path, append(encoded, '\n'), 0o644)
}

func containsMarker(entries []any) bool {
	encoded, err := json.Marshal(entries)
	if err != nil {
		return false
	}
	return strings.Contains(string(encoded), hookMarker)
}
