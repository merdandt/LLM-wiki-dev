package hook

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/merdandt/LLM-wiki-dev/internal/state"
)

func TestSessionStartUninitializedIsSilent(t *testing.T) {
	result, err := SessionStart(context.Background(), Input{
		SessionID: "s1", CWD: t.TempDir(), EventName: "SessionStart",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Context != "" {
		t.Fatalf("expected silence, got %q", result.Context)
	}
}

func TestSessionStartWritesBaselineAndPacket(t *testing.T) {
	root := initializedRepoFixture(t)
	result, err := SessionStart(context.Background(), Input{
		SessionID: "s1", CWD: root, EventName: "SessionStart", Source: "startup",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Context) == 0 || len(result.Context) > 1024 {
		t.Fatalf("packet size %d, want 1..1024", len(result.Context))
	}
	for _, want := range []string{"docs/llm-wiki", "index.md", "status", "validate"} {
		if !strings.Contains(result.Context, want) {
			t.Fatalf("packet lacks %q:\n%s", want, result.Context)
		}
	}
	layout := state.NewLayout(filepath.Join(root, ".llm-wiki-state"))
	session, err := layout.ReadSession("s1")
	if err != nil {
		t.Fatal(err)
	}
	if session.Baseline.Evidence == "" || session.Baseline.Wiki == "" {
		t.Fatalf("baseline fingerprint incomplete: %#v", session.Baseline)
	}
}

func TestSessionStartUnseenCommits(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		content   string
		wantAudit bool
	}{
		{name: "material commit", path: "internal/orders/service.go", content: "package orders\n", wantAudit: true},
		{name: "non-material commit", path: "NOTES.txt", content: "notes\n", wantAudit: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := initializedRepoFixture(t)
			repo := discoverRepo(t, root)
			head, err := repo.Head()
			if err != nil {
				t.Fatal(err)
			}
			worktreeID, err := repo.WorktreeID()
			if err != nil {
				t.Fatal(err)
			}
			layout := state.NewLayout(filepath.Join(root, ".llm-wiki-state"))
			if err := layout.WriteObservation(state.Observation{WorktreeID: worktreeID, Head: head}); err != nil {
				t.Fatal(err)
			}
			writeAndCommit(t, root, tt.path, tt.content)
			if _, err := SessionStart(context.Background(), Input{
				SessionID: "s1", CWD: root, EventName: "SessionStart",
			}); err != nil {
				t.Fatal(err)
			}
			session, err := layout.ReadSession("s1")
			if err != nil {
				t.Fatal(err)
			}
			if session.StartupAudit != tt.wantAudit {
				t.Fatalf("StartupAudit = %v, want %v", session.StartupAudit, tt.wantAudit)
			}
		})
	}
}

func TestSessionStartScaffoldEmitsOnboarding(t *testing.T) {
	root := initializedRepoFixture(t)
	data, err := os.ReadFile(filepath.Join(root, "llm-wiki.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(data), "initialized: true", "initialized: false", 1)
	if err := os.WriteFile(filepath.Join(root, "llm-wiki.yaml"), []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := SessionStart(context.Background(), Input{
		SessionID: "s1", CWD: root, EventName: "SessionStart", Source: "startup",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Context) == 0 || len(result.Context) > 1024 {
		t.Fatalf("onboarding packet size %d, want 1..1024", len(result.Context))
	}
	for _, want := range []string{"not yet compiled", "finalize-init", "docs/llm-wiki"} {
		if !strings.Contains(result.Context, want) {
			t.Fatalf("onboarding packet lacks %q:\n%s", want, result.Context)
		}
	}
	layout := state.NewLayout(filepath.Join(root, ".llm-wiki-state"))
	if _, err := layout.ReadSession("s1"); err == nil {
		t.Fatal("scaffold session-start must not write session state")
	}
}
