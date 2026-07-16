package wiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePage(t *testing.T) {
	input := []byte(`---
id: component.auth
kind: component
status: current
summary: Auth component.
verification:
  base_commit: abc
  evidence_fingerprint: sha256:def
evidence:
  - path: src/auth.go
relations:
  - flow.login
---
# Auth

See [Login](../flows/login.md).
`)

	page, err := ParsePage("components/auth.md", input)
	if err != nil {
		t.Fatal(err)
	}
	if page.ID != "component.auth" || page.Kind != "component" {
		t.Fatalf("unexpected page: %#v", page)
	}
	if len(page.Links) != 1 || page.Links[0] != "../flows/login.md" {
		t.Fatalf("unexpected links: %#v", page.Links)
	}
	crlf := []byte(stringBytesCRLF(input))
	other, err := ParsePage("components/auth.md", crlf)
	if err != nil {
		t.Fatal(err)
	}
	if other.ID != page.ID || other.Kind != page.Kind || string(other.Body) != string(page.Body) ||
		len(other.Links) != len(page.Links) || other.Links[0] != page.Links[0] {
		t.Fatalf("CRLF page differs: %#v vs %#v", other, page)
	}
}

func TestExtractLinksIgnoresImagesAndCapturesReferenceLinks(t *testing.T) {
	links := ExtractLinks([]byte("[one](one.md) ![image](image.png)\n[ref][two]\n\n[two]: two.md\n"))
	if len(links) != 2 || links[0] != "one.md" || links[1] != "two.md" {
		t.Fatalf("ExtractLinks = %#v", links)
	}
}

func stringBytesCRLF(input []byte) []byte {
	var output []byte
	for _, byteValue := range input {
		if byteValue == '\n' {
			output = append(output, '\r')
		}
		output = append(output, byteValue)
	}
	return output
}

func TestValidateValidWiki(t *testing.T) {
	root := copyFixture(t, "../../testdata/wiki/valid")
	report := Validate(Options{Root: root, WikiPath: "docs/llm-wiki", AllowUninitialized: false})
	if len(report.Errors) != 0 {
		t.Fatalf("errors = %#v", report.Errors)
	}
}

func TestValidateDuplicateID(t *testing.T) {
	root := copyFixture(t, "../../testdata/wiki/valid")
	writeFixture(t, root, "docs/llm-wiki/duplicate.md", strings.ReplaceAll(readFixture(t, root, "docs/llm-wiki/system.md"), "system.root", "index.root"))
	report := Validate(Options{Root: root, WikiPath: "docs/llm-wiki"})
	assertIssueCode(t, report, "duplicate-id")
}

func TestValidateMissingRequiredFile(t *testing.T) {
	root := copyFixture(t, "../../testdata/wiki/valid")
	if err := os.Remove(filepath.Join(root, "docs/llm-wiki/system.md")); err != nil {
		t.Fatal(err)
	}
	assertIssueCode(t, Validate(Options{Root: root, WikiPath: "docs/llm-wiki"}), "missing-required-file")
}

func TestValidateUnknownFrontmatterField(t *testing.T) {
	root := copyFixture(t, "../../testdata/wiki/valid")
	writeFixture(t, root, "docs/llm-wiki/system.md", strings.Replace(readFixture(t, root, "docs/llm-wiki/system.md"), "evidence: []", "unknown: true\nevidence: []", 1))
	assertIssueCode(t, Validate(Options{Root: root, WikiPath: "docs/llm-wiki"}), "frontmatter")
}

func TestValidateInvalidKindAndStatus(t *testing.T) {
	root := copyFixture(t, "../../testdata/wiki/valid")
	data := readFixture(t, root, "docs/llm-wiki/system.md")
	data = strings.Replace(data, "kind: system", "kind: unknown", 1)
	data = strings.Replace(data, "status: current", "status: unknown", 1)
	writeFixture(t, root, "docs/llm-wiki/system.md", data)
	report := Validate(Options{Root: root, WikiPath: "docs/llm-wiki"})
	assertIssueCode(t, report, "invalid-kind")
	assertIssueCode(t, report, "invalid-status")
}

func TestValidateBrokenLinkAndOrphanPage(t *testing.T) {
	root := copyFixture(t, "../../testdata/wiki/valid")
	data := readFixture(t, root, "docs/llm-wiki/index.md")
	data = strings.Replace(data, "- [System](system.md)", "- [System](missing.md)", 1)
	writeFixture(t, root, "docs/llm-wiki/index.md", data)
	writeFixture(t, root, "docs/llm-wiki/orphan.md", strings.Replace(readFixture(t, root, "docs/llm-wiki/system.md"), "system.root", "orphan", 1))
	report := Validate(Options{Root: root, WikiPath: "docs/llm-wiki"})
	assertIssueCode(t, report, "broken-link")
	assertIssueCode(t, report, "orphan-page")
}

func TestValidateStaleEvidenceFingerprint(t *testing.T) {
	root := copyFixture(t, "../../testdata/wiki/valid")
	writeFixture(t, root, "evidence.txt", "actual evidence\n")
	data := readFixture(t, root, "docs/llm-wiki/system.md")
	data = strings.Replace(data, "evidence: []", "evidence:\n  - path: evidence.txt", 1)
	data = strings.Replace(data, "sha256:0000000000000000000000000000000000000000000000000000000000000000", "sha256:1111111111111111111111111111111111111111111111111111111111111111", 1)
	writeFixture(t, root, "docs/llm-wiki/system.md", data)
	assertIssueCode(t, Validate(Options{Root: root, WikiPath: "docs/llm-wiki"}), "stale-evidence")
}

func TestValidateUnsafeEvidencePathAndSymlink(t *testing.T) {
	root := copyFixture(t, "../../testdata/wiki/valid")
	data := readFixture(t, root, "docs/llm-wiki/system.md")
	data = strings.Replace(data, "evidence: []", "evidence:\n  - path: ../secret.txt", 1)
	writeFixture(t, root, "docs/llm-wiki/system.md", data)
	assertIssueCode(t, Validate(Options{Root: root, WikiPath: "docs/llm-wiki"}), "unsafe-evidence-path")

	root = copyFixture(t, "../../testdata/wiki/valid")
	writeFixture(t, root, "real-evidence.txt", "evidence\n")
	if err := os.Symlink(filepath.Join(root, "real-evidence.txt"), filepath.Join(root, "evidence-link.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	data = strings.Replace(readFixture(t, root, "docs/llm-wiki/system.md"), "evidence: []", "evidence:\n  - path: evidence-link.txt", 1)
	writeFixture(t, root, "docs/llm-wiki/system.md", data)
	assertIssueCode(t, Validate(Options{Root: root, WikiPath: "docs/llm-wiki"}), "unsupported-evidence-type")
}

func TestValidateSymlinkedWikiRootAndPage(t *testing.T) {
	root := copyFixture(t, "../../testdata/wiki/valid")
	wikiPath := filepath.Join(root, "docs/llm-wiki")
	if err := os.Rename(wikiPath, filepath.Join(root, "real-wiki")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "real-wiki"), wikiPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	assertIssueCode(t, Validate(Options{Root: root, WikiPath: "docs/llm-wiki"}), "unsafe-wiki-root")

	root = copyFixture(t, "../../testdata/wiki/valid")
	system := filepath.Join(root, "docs/llm-wiki/system.md")
	if err := os.Rename(system, filepath.Join(root, "real-system.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "real-system.md"), system); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	assertIssueCode(t, Validate(Options{Root: root, WikiPath: "docs/llm-wiki"}), "unsafe-wiki-file")
}

func TestValidateSupersessionCycleAndStatuses(t *testing.T) {
	root := copyFixture(t, "../../testdata/wiki/valid")
	base := readFixture(t, root, "docs/llm-wiki/system.md")
	decisionA := strings.ReplaceAll(base, "system.root", "decision.a")
	decisionA = strings.Replace(decisionA, "kind: system", "kind: decision", 1)
	decisionA = strings.Replace(decisionA, "summary: System overview.", "summary: Decision A.", 1)
	decisionA = strings.Replace(decisionA, "relations: []", "supersedes:\n  - decision.b", 1)
	decisionB := strings.ReplaceAll(base, "system.root", "decision.b")
	decisionB = strings.Replace(decisionB, "kind: system", "kind: decision", 1)
	decisionB = strings.Replace(decisionB, "summary: System overview.", "summary: Decision B.", 1)
	decisionB = strings.Replace(decisionB, "relations: []", "supersedes:\n  - decision.a", 1)
	writeFixture(t, root, "docs/llm-wiki/decision-a.md", decisionA)
	writeFixture(t, root, "docs/llm-wiki/decision-b.md", decisionB)
	index := readFixture(t, root, "docs/llm-wiki/index.md") + "\n- [Decision A](decision-a.md)\n- [Decision B](decision-b.md)\n"
	writeFixture(t, root, "docs/llm-wiki/index.md", index)
	assertIssueCode(t, Validate(Options{Root: root, WikiPath: "docs/llm-wiki"}), "supersession-cycle")
}

func TestValidateInvalidLogHeading(t *testing.T) {
	root := copyFixture(t, "../../testdata/wiki/valid")
	data := readFixture(t, root, "docs/llm-wiki/log.md") + "\n## invalid heading\n"
	writeFixture(t, root, "docs/llm-wiki/log.md", data)
	assertIssueCode(t, Validate(Options{Root: root, WikiPath: "docs/llm-wiki"}), "invalid-log-heading")
}

func TestValidateIndexEntryLimit(t *testing.T) {
	root := copyFixture(t, "../../testdata/wiki/valid")
	assertIssueCode(t, Validate(Options{Root: root, WikiPath: "docs/llm-wiki", IndexEntryLimit: 1}), "index-entry-limit")
}

func TestValidateLikelySecret(t *testing.T) {
	root := copyFixture(t, "../../testdata/wiki/valid")
	path := filepath.Join(root, "docs/llm-wiki/schema.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("\nToken: ghp_1234567890123456789012345678901234\n")...)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	assertIssueCode(t, Validate(Options{Root: root, WikiPath: "docs/llm-wiki"}), "likely-secret")
}

func assertIssueCode(t *testing.T, report Report, code string) {
	t.Helper()
	if !report.ContainsCode(code) {
		t.Fatalf("errors = %#v, want %s", report.Errors, code)
	}
}

func copyFixture(t *testing.T, relative string) string {
	t.Helper()
	source := relative
	target := t.TempDir()
	if err := copyDirectory(source, target); err != nil {
		t.Fatal(err)
	}
	return target
}

func copyDirectory(source, target string) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(source, entry.Name())
		targetPath := filepath.Join(target, entry.Name())
		if entry.IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
			if err := copyDirectory(sourcePath, targetPath); err != nil {
				return err
			}
			continue
		}
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func readFixture(t *testing.T, root, relative string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeFixture(t *testing.T, root, relative, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
