package materiality

import (
	"path/filepath"
	"strings"
)

type Hint string

const (
	HintNone     Hint = "none"
	HintReview   Hint = "review"
	HintPossible Hint = "possible"
)

func ClassifyPaths(paths []string, wikiPath string) Hint {
	result := HintNone
	wikiRoot := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(wikiPath)), "/")
	for _, path := range paths {
		slash := filepath.ToSlash(path)
		if slash == wikiRoot || strings.HasPrefix(slash, wikiRoot+"/") {
			continue
		}
		base := filepath.Base(slash)
		switch base {
		case "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "go.sum", "Cargo.lock", "pubspec.lock":
			if result == HintNone {
				result = HintReview
			}
			continue
		}
		ext := strings.ToLower(filepath.Ext(base))
		switch ext {
		case ".go", ".rs", ".py", ".js", ".jsx", ".ts", ".tsx", ".java", ".kt", ".swift",
			".dart", ".rb", ".php", ".cs", ".sql", ".proto", ".graphql", ".yaml", ".yml",
			".toml", ".json":
			return HintPossible
		case ".md", ".mdx", ".txt", ".png", ".jpg", ".jpeg", ".gif", ".svg":
			continue
		default:
			if result == HintNone {
				result = HintReview
			}
		}
	}
	return result
}
