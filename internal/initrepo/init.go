package initrepo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/merdandt/LLM-wiki-dev/internal/atomicfile"
	"github.com/merdandt/LLM-wiki-dev/internal/gitrepo"
)

const (
	startMarker = "<!-- llm-wiki:start -->"
	endMarker   = "<!-- llm-wiki:end -->"
)

// Initialize installs the tracked release template without overwriting
// existing project files. It is intentionally local-only and idempotent.
func Initialize(root, templateDir string) error {
	repo, err := gitrepo.Discover(root)
	if err != nil {
		return err
	}
	if templateDir == "" {
		templateDir = os.Getenv("LLM_WIKI_TEMPLATE_DIR")
	}
	if templateDir == "" {
		templateDir = filepath.Join(repo.Root, "template")
	}
	templateDir, err = filepath.Abs(templateDir)
	if err != nil {
		return err
	}
	info, err := os.Stat(templateDir)
	if err != nil || !info.IsDir() {
		return errors.New("llm-wiki template directory is unavailable")
	}
	projectName := filepath.Base(repo.Root)
	replacements := strings.NewReplacer(
		"{{project_name}}", projectName,
		"{{wiki_path}}", "docs/llm-wiki",
		"{{context_budget_bytes}}", "12288",
		"{{current_date}}", time.Now().UTC().Format("2006-01-02"),
	)
	return fs.WalkDir(os.DirFS(templateDir), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || path == "." {
			return nil
		}
		if path == ".gitignore.append" {
			data, err := os.ReadFile(filepath.Join(templateDir, path))
			if err != nil {
				return err
			}
			return mergeGitignore(repo.Root, string(data))
		}
		data, err := os.ReadFile(filepath.Join(templateDir, path))
		if err != nil {
			return err
		}
		data = []byte(replacements.Replace(string(data)))
		destination := filepath.Join(repo.Root, filepath.FromSlash(path))
		if filepath.Base(path) == "AGENTS.md" || filepath.Base(path) == "CLAUDE.md" {
			return mergeInstructions(destination, string(data))
		}
		if _, err := os.Lstat(destination); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return atomicfile.Write(destination, data, 0o644)
	})
}

func mergeGitignore(root, addition string) error {
	path := filepath.Join(root, ".gitignore")
	existing, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return atomicfile.Write(path, []byte(strings.TrimRight(addition, "\n")+"\n"), 0o644)
	}
	if err != nil {
		return err
	}
	if strings.Contains(string(existing), strings.TrimSpace(addition)) {
		return nil
	}
	joined := strings.TrimRight(string(existing), "\n") + "\n" + strings.TrimRight(addition, "\n") + "\n"
	return atomicfile.Write(path, []byte(joined), 0o644)
}

func mergeInstructions(path, managed string) error {
	existing, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return atomicfile.Write(path, []byte(managed), 0o644)
	}
	if err != nil {
		return err
	}
	text := string(existing)
	if strings.Contains(text, managed) {
		return nil
	}
	start, end := strings.Index(text, startMarker), strings.Index(text, endMarker)
	if start >= 0 && end > start {
		end += len(endMarker)
		text = text[:start] + managed + text[end:]
	} else {
		text = strings.TrimRight(text, "\n") + "\n\n" + managed + "\n"
	}
	return atomicfile.Write(path, []byte(text), 0o644)
}

func ValidateTemplate(templateDir string) error {
	if templateDir == "" {
		return fmt.Errorf("template directory is required")
	}
	_, err := os.Stat(filepath.Join(templateDir, "llm-wiki.yaml"))
	return err
}
