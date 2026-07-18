package hook

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/merdandt/LLM-wiki-dev/internal/config"
	"github.com/merdandt/LLM-wiki-dev/internal/fingerprint"
	"github.com/merdandt/LLM-wiki-dev/internal/gitrepo"
	"github.com/merdandt/LLM-wiki-dev/internal/state"
)

func BuildFingerprint(repo gitrepo.Repo, cfg config.Config) (state.Fingerprint, error) {
	head, err := repo.Head()
	if err != nil {
		return state.Fingerprint{}, err
	}
	patch, err := repo.WorktreePatch(cfg.WikiPath, cfg.StatePath, "llm-wiki.yaml")
	if err != nil {
		return state.Fingerprint{}, err
	}
	evidence := fingerprint.Records([]fingerprint.Record{{
		Path: "@worktree-patch",
		Kind: "git-diff",
		Data: []byte(patch),
	}})

	wikiOutput, err := repo.Output(
		"ls-files", "-co", "--exclude-standard", "--", "llm-wiki.yaml", cfg.WikiPath)
	if err != nil {
		return state.Fingerprint{}, err
	}
	var wikiRecords []fingerprint.Record
	for _, relative := range strings.Split(wikiOutput, "\n") {
		if relative == "" {
			continue
		}
		if relative != "llm-wiki.yaml" && !strings.EqualFold(filepath.Ext(relative), ".md") {
			continue
		}
		record, err := fileRecord(repo.Root, relative)
		if err != nil {
			return state.Fingerprint{}, err
		}
		wikiRecords = append(wikiRecords, record)
	}
	return state.Fingerprint{
		BaseCommit: head,
		Evidence:   evidence,
		Wiki:       fingerprint.Records(wikiRecords),
		Schema:     cfg.SchemaVersion,
	}, nil
}

func fileRecord(root, relative string) (fingerprint.Record, error) {
	slash := filepath.ToSlash(filepath.Clean(relative))
	full := filepath.Join(root, filepath.FromSlash(slash))
	info, err := os.Lstat(full)
	if os.IsNotExist(err) {
		return fingerprint.Record{Path: slash, Kind: "missing"}, nil
	}
	if err != nil {
		return fingerprint.Record{}, err
	}
	switch {
	case info.Mode().IsRegular():
		data, err := os.ReadFile(full)
		return fingerprint.Record{Path: slash, Kind: "file", Data: data}, err
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(full)
		return fingerprint.Record{Path: slash, Kind: "symlink", Data: []byte(filepath.ToSlash(target))}, err
	default:
		return fingerprint.Record{Path: slash, Kind: info.Mode().String()}, nil
	}
}

func inside(relative, directory string) bool {
	path := filepath.ToSlash(filepath.Clean(relative))
	root := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(directory)), "/")
	return path == root || strings.HasPrefix(path, root+"/")
}
