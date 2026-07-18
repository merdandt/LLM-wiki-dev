package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/merdandt/LLM-wiki-dev/internal/hook"
)

// runHook adapts stdin/stdout to the hook state machines. It always exits 0:
// a broken hook must never break the agent session.
func runHook(stdin io.Reader, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || (args[0] != "session-start" && args[0] != "stop") {
		fmt.Fprintln(stderr, "usage: llm-wiki hook <session-start|stop>")
		return 2
	}
	data, err := io.ReadAll(io.LimitReader(stdin, 1<<20))
	if err != nil {
		fmt.Fprintf(stderr, "llm-wiki: hook input unreadable: %v\n", err)
		return 0
	}
	input, err := hook.Decode(data)
	if err != nil {
		fmt.Fprintf(stderr, "llm-wiki: hook input rejected: %v\n", err)
		return 0
	}
	wantEvent := map[string]string{"session-start": "SessionStart", "stop": "Stop"}[args[0]]
	if input.EventName != wantEvent {
		fmt.Fprintf(stderr, "llm-wiki: hook event %q does not match subcommand %q\n", input.EventName, args[0])
		return 0
	}
	var result hook.Result
	if wantEvent == "SessionStart" {
		result, err = hook.SessionStart(context.Background(), input)
	} else {
		result, err = hook.Stop(context.Background(), input)
	}
	if err != nil {
		fmt.Fprintf(stderr, "llm-wiki: hook error: %v\n", err)
		return 0
	}
	if result.Outcome == hook.OutcomeFailure && result.Reason != "" {
		fmt.Fprintln(stderr, result.Reason)
	}
	if err := hook.Encode(wantEvent, result, stdout); err != nil {
		fmt.Fprintf(stderr, "llm-wiki: hook output failed: %v\n", err)
	}
	return 0
}
