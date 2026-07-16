package wiki

import (
	"bytes"
	"errors"

	"gopkg.in/yaml.v3"
)

type Verification struct {
	BaseCommit          string `yaml:"base_commit" json:"base_commit"`
	EvidenceFingerprint string `yaml:"evidence_fingerprint" json:"evidence_fingerprint"`
}

type Evidence struct {
	Path   string `yaml:"path" json:"path"`
	Symbol string `yaml:"symbol,omitempty" json:"symbol,omitempty"`
}

type Page struct {
	Path         string       `yaml:"-" json:"path"`
	ID           string       `yaml:"id" json:"id"`
	Kind         string       `yaml:"kind" json:"kind"`
	Status       string       `yaml:"status" json:"status"`
	Summary      string       `yaml:"summary" json:"summary"`
	Verification Verification `yaml:"verification" json:"verification"`
	Evidence     []Evidence   `yaml:"evidence" json:"evidence"`
	Relations    []string     `yaml:"relations,omitempty" json:"relations,omitempty"`
	Supersedes   []string     `yaml:"supersedes,omitempty" json:"supersedes,omitempty"`
	Links        []string     `yaml:"-" json:"links"`
	Body         []byte       `yaml:"-" json:"-"`
}

func ParsePage(path string, data []byte) (Page, error) {
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return Page{}, errors.New("missing YAML frontmatter")
	}
	end := bytes.Index(data[4:], []byte("\n---\n"))
	if end < 0 {
		return Page{}, errors.New("unterminated YAML frontmatter")
	}
	frontmatter := data[4 : 4+end]
	body := data[4+end+5:]
	page := Page{Path: path}
	decoder := yaml.NewDecoder(bytes.NewReader(frontmatter))
	decoder.KnownFields(true)
	if err := decoder.Decode(&page); err != nil {
		return Page{}, err
	}
	page.Links = ExtractLinks(body)
	page.Body = append([]byte(nil), body...)
	return page, nil
}
