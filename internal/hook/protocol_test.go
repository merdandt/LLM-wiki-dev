package hook

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDecode(t *testing.T) {
	valid := []byte(`{"session_id":"s1","cwd":"/tmp/p","hook_event_name":"Stop","stop_hook_active":true}`)
	input, err := Decode(valid)
	if err != nil {
		t.Fatal(err)
	}
	if input.SessionID != "s1" || input.CWD != "/tmp/p" || input.EventName != "Stop" || !input.StopHookActive {
		t.Fatalf("unexpected input: %#v", input)
	}
	for name, data := range map[string]string{
		"missing session": `{"cwd":"/tmp/p","hook_event_name":"Stop"}`,
		"unknown event":   `{"session_id":"s1","cwd":"/tmp/p","hook_event_name":"Notification"}`,
		"malformed":       `{`,
	} {
		if _, err := Decode([]byte(data)); err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
}

func TestEncodeSessionStartPrintsContext(t *testing.T) {
	var out bytes.Buffer
	err := Encode("SessionStart", Result{Outcome: OutcomeClean, Context: "LLM Wiki: memory at docs/llm-wiki/."}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "LLM Wiki") {
		t.Fatalf("context not printed: %q", out.String())
	}
}

func TestEncodeSilentOutcomes(t *testing.T) {
	for _, result := range []Result{
		{Outcome: OutcomeClean},
		{Outcome: OutcomeSynchronized},
		{Outcome: OutcomeFailure, Reason: "warning goes to stderr, not stdout"},
	} {
		var out bytes.Buffer
		if err := Encode("Stop", result, &out); err != nil {
			t.Fatal(err)
		}
		if out.Len() != 0 {
			t.Fatalf("outcome %q wrote stdout: %q", result.Outcome, out.String())
		}
	}
}

func TestEncodeDriftBlocksWithReason(t *testing.T) {
	var out bytes.Buffer
	if err := Encode("Stop", Result{Outcome: OutcomeDrift, Reason: "sync the wiki"}, &out); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["decision"] != "block" || payload["reason"] != "sync the wiki" {
		t.Fatalf("unexpected payload: %v", payload)
	}
}
