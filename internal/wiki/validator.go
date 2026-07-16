package wiki

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/merdandt/LLM-wiki-dev/internal/fingerprint"
)

type Issue struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

type Report struct {
	Errors   []Issue `json:"errors"`
	Warnings []Issue `json:"warnings"`
}

func (r Report) ContainsCode(code string) bool {
	for _, issue := range r.Errors {
		if issue.Code == code {
			return true
		}
	}
	return false
}

type Options struct {
	Root               string
	WikiPath           string
	AllowUninitialized bool
	IndexEntryLimit    int
}

func Validate(options Options) Report {
	var report Report
	if options.IndexEntryLimit == 0 {
		options.IndexEntryLimit = 200
	}
	wikiRoot := filepath.Join(options.Root, filepath.FromSlash(options.WikiPath))
	pages := map[string]Page{}
	ids := map[string]string{}
	var pagePaths []string
	if err := validateWikiDirectoryChain(options.Root, options.WikiPath); err != nil {
		report.Errors = append(report.Errors, Issue{Code: "unsafe-wiki-root", Path: options.WikiPath, Message: err.Error()})
		return report
	}
	validateRequiredFiles(&report, wikiRoot)
	if data, err := os.ReadFile(filepath.Join(options.Root, "llm-wiki.yaml")); err == nil {
		if matches := SecretMatches(data); len(matches) > 0 {
			report.Errors = append(report.Errors, Issue{Code: "likely-secret", Path: "llm-wiki.yaml", Message: strings.Join(matches, ", ")})
		}
	}

	_ = filepath.WalkDir(wikiRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			report.Errors = append(report.Errors, Issue{Code: "walk", Path: path, Message: walkErr.Error()})
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if filepath.Ext(path) == ".md" {
				report.Errors = append(report.Errors, Issue{Code: "unsafe-wiki-file", Path: path})
			}
			return nil
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			report.Errors = append(report.Errors, Issue{Code: "stat", Path: path, Message: err.Error()})
			return nil
		}
		if !info.Mode().IsRegular() {
			report.Errors = append(report.Errors, Issue{Code: "unsafe-wiki-file", Path: path})
			return nil
		}
		relative, _ := filepath.Rel(wikiRoot, path)
		relative = filepath.ToSlash(relative)
		data, err := os.ReadFile(path)
		if err != nil {
			report.Errors = append(report.Errors, Issue{Code: "read", Path: relative, Message: err.Error()})
			return nil
		}
		if matches := SecretMatches(data); len(matches) > 0 {
			report.Errors = append(report.Errors, Issue{Code: "likely-secret", Path: relative, Message: strings.Join(matches, ", ")})
		}
		if strings.HasPrefix(entry.Name(), "_") {
			return nil
		}
		page, err := ParsePage(relative, data)
		if err != nil {
			report.Errors = append(report.Errors, Issue{Code: "frontmatter", Path: relative, Message: err.Error()})
			return nil
		}
		if previous, exists := ids[page.ID]; exists {
			report.Errors = append(report.Errors, Issue{Code: "duplicate-id", Path: relative, Message: fmt.Sprintf("also used by %s", previous)})
		}
		ids[page.ID] = relative
		pages[relative] = page
		pagePaths = append(pagePaths, relative)
		return nil
	})

	sort.Strings(pagePaths)
	for _, relative := range pagePaths {
		page := pages[relative]
		validatePageMetadata(&report, page, options.AllowUninitialized)
		for _, evidence := range page.Evidence {
			clean := filepath.Clean(filepath.FromSlash(evidence.Path))
			if evidence.Path == "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				report.Errors = append(report.Errors, Issue{Code: "unsafe-evidence-path", Path: relative, Message: evidence.Path})
				continue
			}
			info, err := os.Lstat(filepath.Join(options.Root, clean))
			if err != nil {
				report.Errors = append(report.Errors, Issue{Code: "missing-evidence", Path: relative, Message: evidence.Path})
			} else if !info.Mode().IsRegular() {
				report.Errors = append(report.Errors, Issue{Code: "unsupported-evidence-type", Path: relative, Message: evidence.Path})
			}
		}
		for _, link := range page.Links {
			linkPath, local := localLinkPath(link)
			if !local {
				continue
			}
			target := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(relative), filepath.FromSlash(linkPath))))
			if _, ok := pages[target]; !ok {
				report.Errors = append(report.Errors, Issue{Code: "broken-link", Path: relative, Message: link})
			}
		}
		for _, supersededID := range page.Supersedes {
			if page.Status != "current" {
				report.Errors = append(report.Errors, Issue{Code: "superseding-status-mismatch", Path: relative, Message: page.Status})
			}
			supersededPath, ok := ids[supersededID]
			if !ok {
				report.Errors = append(report.Errors, Issue{Code: "missing-superseded-id", Path: relative, Message: supersededID})
			} else if page.Kind != "decision" || pages[supersededPath].Kind != "decision" {
				report.Errors = append(report.Errors, Issue{Code: "invalid-supersession-kind", Path: relative, Message: supersededID})
			} else if pages[supersededPath].Status != "superseded" && pages[supersededPath].Status != "current" {
				report.Errors = append(report.Errors, Issue{Code: "superseded-status-mismatch", Path: supersededPath, Message: page.ID})
			}
		}
		for _, relatedID := range page.Relations {
			if _, ok := ids[relatedID]; !ok {
				report.Errors = append(report.Errors, Issue{Code: "missing-relation-id", Path: relative, Message: relatedID})
			}
		}
		validateEvidenceFingerprint(&report, options.Root, page)
		if page.Kind == "log" {
			validateLogHeadings(&report, page)
		}
	}
	validateIndexCoverage(&report, pages, pagePaths, options.IndexEntryLimit)
	validateSupersessionCycles(&report, pages, ids)
	return report
}

var validKinds = map[string]struct{}{"system": {}, "component": {}, "flow": {}, "contract": {}, "decision": {}, "quality": {}, "operation": {}, "glossary": {}, "health": {}, "index": {}, "log": {}}
var validStatuses = map[string]struct{}{"current": {}, "deprecated": {}, "superseded": {}, "planned": {}}
var stableID = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*$`)
var evidenceFingerprint = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

func validatePageMetadata(report *Report, page Page, allowUninitialized bool) {
	required := []struct{ value, code string }{{page.ID, "missing-id"}, {page.Kind, "missing-kind"}, {page.Status, "missing-status"}, {page.Summary, "missing-summary"}, {page.Verification.BaseCommit, "missing-base-commit"}, {page.Verification.EvidenceFingerprint, "missing-evidence-fingerprint"}}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			report.Errors = append(report.Errors, Issue{Code: field.code, Path: page.Path})
		}
	}
	if page.Evidence == nil {
		report.Errors = append(report.Errors, Issue{Code: "missing-evidence-list", Path: page.Path})
	}
	if page.ID != "" && !stableID.MatchString(page.ID) {
		report.Errors = append(report.Errors, Issue{Code: "invalid-id", Path: page.Path, Message: page.ID})
	}
	if len(page.Summary) > 300 {
		report.Errors = append(report.Errors, Issue{Code: "summary-too-long", Path: page.Path})
	}
	if _, ok := validKinds[page.Kind]; page.Kind != "" && !ok {
		report.Errors = append(report.Errors, Issue{Code: "invalid-kind", Path: page.Path, Message: page.Kind})
	}
	if _, ok := validStatuses[page.Status]; page.Status != "" && !ok {
		report.Errors = append(report.Errors, Issue{Code: "invalid-status", Path: page.Path, Message: page.Status})
	}
	if page.Verification.EvidenceFingerprint == "uninitialized" {
		issue := Issue{Code: "uninitialized-evidence", Path: page.Path}
		if allowUninitialized {
			report.Warnings = append(report.Warnings, issue)
		} else {
			report.Errors = append(report.Errors, issue)
		}
	} else if page.Verification.EvidenceFingerprint != "" && !evidenceFingerprint.MatchString(page.Verification.EvidenceFingerprint) {
		report.Errors = append(report.Errors, Issue{Code: "invalid-evidence-fingerprint", Path: page.Path, Message: page.Verification.EvidenceFingerprint})
	}
	for _, id := range append(append([]string(nil), page.Relations...), page.Supersedes...) {
		if !stableID.MatchString(id) {
			report.Errors = append(report.Errors, Issue{Code: "invalid-related-id", Path: page.Path, Message: id})
		}
	}
}

func validateEvidenceFingerprint(report *Report, root string, page Page) {
	if page.Verification.EvidenceFingerprint == "" || page.Verification.EvidenceFingerprint == "uninitialized" || len(page.Evidence) == 0 {
		return
	}
	paths := make([]string, 0, len(page.Evidence))
	for _, item := range page.Evidence {
		clean := filepath.Clean(filepath.FromSlash(item.Path))
		if item.Path == "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return
		}
		info, err := os.Lstat(filepath.Join(root, clean))
		if err != nil || !info.Mode().IsRegular() {
			return
		}
		paths = append(paths, item.Path)
	}
	got, err := fingerprint.Files(root, paths)
	if err != nil {
		report.Errors = append(report.Errors, Issue{Code: "fingerprint-error", Path: page.Path, Message: err.Error()})
		return
	}
	if got != page.Verification.EvidenceFingerprint {
		report.Errors = append(report.Errors, Issue{Code: "stale-evidence", Path: page.Path, Message: fmt.Sprintf("got %s, want %s", got, page.Verification.EvidenceFingerprint)})
	}
}

func validateWikiDirectoryChain(root, wikiPath string) error {
	current := root
	for _, segment := range strings.Split(filepath.ToSlash(wikiPath), "/") {
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("unsafe path component: %s", current)
		}
	}
	return nil
}

func validateRequiredFiles(report *Report, wikiRoot string) {
	for _, relative := range []string{"index.md", "system.md", "schema.md", "glossary.md", "health.md", "log.md", "quality/invariants.md", "quality/testing.md", "quality/failure-modes.md"} {
		info, err := os.Lstat(filepath.Join(wikiRoot, filepath.FromSlash(relative)))
		if err != nil || !info.Mode().IsRegular() {
			report.Errors = append(report.Errors, Issue{Code: "missing-required-file", Path: relative})
		}
	}
}

var logHeading = regexp.MustCompile(`^## \[\d{4}-\d{2}-\d{2}\] (init|sync|audit|migrate) \| .+$`)

func validateLogHeadings(report *Report, page Page) {
	for _, line := range strings.Split(string(page.Body), "\n") {
		if strings.HasPrefix(line, "## ") && !logHeading.MatchString(line) {
			report.Errors = append(report.Errors, Issue{Code: "invalid-log-heading", Path: page.Path, Message: line})
		}
	}
}

func validateIndexCoverage(report *Report, pages map[string]Page, pagePaths []string, entryLimit int) {
	linked := map[string]struct{}{}
	for _, relative := range pagePaths {
		page := pages[relative]
		if page.Kind != "index" {
			continue
		}
		if len(page.Links) > entryLimit {
			report.Errors = append(report.Errors, Issue{Code: "index-entry-limit", Path: relative, Message: fmt.Sprintf("%d links exceeds %d", len(page.Links), entryLimit)})
		}
		for _, link := range page.Links {
			linkPath, local := localLinkPath(link)
			if !local {
				continue
			}
			target := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(relative), filepath.FromSlash(linkPath))))
			linked[target] = struct{}{}
		}
	}
	for _, relative := range pagePaths {
		if pages[relative].Kind == "index" || pages[relative].Kind == "log" {
			continue
		}
		if _, ok := linked[filepath.ToSlash(relative)]; !ok {
			report.Errors = append(report.Errors, Issue{Code: "orphan-page", Path: relative})
		}
	}
}

func localLinkPath(link string) (string, bool) {
	if strings.HasPrefix(link, "#") || strings.HasPrefix(link, "//") {
		return "", false
	}
	parsed, err := url.Parse(link)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.Path == "" {
		return "", false
	}
	path, err := url.PathUnescape(parsed.Path)
	return path, err == nil
}

func validateSupersessionCycles(report *Report, pages map[string]Page, ids map[string]string) {
	byID := make(map[string]Page, len(pages))
	var orderedIDs []string
	for _, page := range pages {
		byID[page.ID] = page
		orderedIDs = append(orderedIDs, page.ID)
	}
	sort.Strings(orderedIDs)
	state := map[string]uint8{}
	var visit func(string) bool
	visit = func(id string) bool {
		switch state[id] {
		case 1:
			return true
		case 2:
			return false
		}
		state[id] = 1
		for _, older := range byID[id].Supersedes {
			if _, exists := ids[older]; exists && visit(older) {
				return true
			}
		}
		state[id] = 2
		return false
	}
	for _, id := range orderedIDs {
		if visit(id) {
			report.Errors = append(report.Errors, Issue{Code: "supersession-cycle", Path: ids[id], Message: id})
			return
		}
	}
}
