package initrepo

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/merdandt/LLM-wiki-dev/internal/atomicfile"
)

const (
	makefileStart = "# --- llm-wiki start ---"
	makefileEnd   = "# --- llm-wiki end ---"
)

const makefileBlock = makefileStart + `
.PHONY: wiki-status wiki-validate wiki-finalize

wiki-status: ## LLM wiki health and sync state
	./.llm-wiki/llm-wiki status

wiki-validate: ## LLM wiki structural check
	./.llm-wiki/llm-wiki validate

wiki-finalize: ## activate lifecycle hooks after the first compile
	./.llm-wiki/llm-wiki finalize-init
` + makefileEnd + "\n"

// WriteMakefileTargets ensures the project Makefile carries the llm-wiki
// convenience targets inside a marked block. The file is created when
// absent; an existing block is replaced in place; content outside the
// markers is never modified.
func WriteMakefileTargets(root string) error {
	path := filepath.Join(root, "Makefile")
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	text := string(existing)
	start := strings.Index(text, makefileStart)
	end := strings.Index(text, makefileEnd)
	var next string
	switch {
	case len(text) == 0:
		next = makefileBlock
	case start >= 0 && end > start:
		next = text[:start] + strings.TrimSuffix(makefileBlock, "\n") + text[end+len(makefileEnd):]
	default:
		next = strings.TrimRight(text, "\n") + "\n\n" + makefileBlock
	}
	if next == text {
		return nil
	}
	return atomicfile.Write(path, []byte(next), 0o644)
}
