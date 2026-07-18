package cli

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/merdandt/LLM-wiki-dev/internal/atomicfile"
	"github.com/merdandt/LLM-wiki-dev/internal/config"
	"github.com/merdandt/LLM-wiki-dev/internal/gitrepo"
	"github.com/merdandt/LLM-wiki-dev/internal/wiki"
)

// runFinalizeInit marks the wiki as compiled once strict validation passes,
// activating the lifecycle hooks. Idempotent: an already-initialized wiki
// exits 0 silently.
func runFinalizeInit(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("finalize-init", flag.ContinueOnError)
	flags.SetOutput(stderr)
	rootFlag := flags.String("root", ".", "Git repository root")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	repo, err := gitrepo.Discover(*rootFlag)
	if err != nil {
		return commandError(stderr, err)
	}
	configPath := filepath.Join(repo.Root, "llm-wiki.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return commandError(stderr, err)
	}
	if cfg.Initialized {
		return 0
	}
	report := wiki.Validate(wiki.Options{
		Root: repo.Root, WikiPath: cfg.WikiPath, IndexEntryLimit: cfg.IndexEntryLimit,
	})
	if len(report.Errors) > 0 {
		fmt.Fprintf(stderr, "llm-wiki: cannot finalize while %d validation errors remain; run `llm-wiki validate`\n", len(report.Errors))
		return 4
	}
	cfg.Initialized = true
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return commandError(stderr, err)
	}
	if err := atomicfile.Write(configPath, data, 0o644); err != nil {
		return commandError(stderr, err)
	}
	fmt.Fprintln(stdout, "llm-wiki: wiki initialized; lifecycle hooks are active")
	return 0
}
