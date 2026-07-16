package wiki

import "testing"

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
