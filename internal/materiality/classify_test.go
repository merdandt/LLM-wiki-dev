package materiality

import "testing"

func TestClassifyPaths(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
		want  Hint
	}{
		{name: "formatting docs only", paths: []string{"README.md"}, want: HintNone},
		{name: "source code", paths: []string{"src/auth/service.ts"}, want: HintPossible},
		{name: "schema", paths: []string{"api/openapi.yaml"}, want: HintPossible},
		{name: "lockfile only", paths: []string{"package-lock.json"}, want: HintReview},
		{name: "extensionless build file", paths: []string{"Dockerfile"}, want: HintReview},
		{name: "wiki only", paths: []string{"docs/llm-wiki/system.md"}, want: HintNone},
		{name: "mixed source and docs", paths: []string{"README.md", "src/auth/service.ts"}, want: HintPossible},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyPaths(tt.paths, "docs/llm-wiki"); got != tt.want {
				t.Fatalf("ClassifyPaths() = %q, want %q", got, tt.want)
			}
		})
	}
}
