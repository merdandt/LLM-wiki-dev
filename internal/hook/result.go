package hook

import (
	"encoding/json"
	"io"
)

type Outcome string

const (
	OutcomeClean        Outcome = "clean"
	OutcomeSynchronized Outcome = "synchronized"
	OutcomeDrift        Outcome = "drift"
	OutcomeFailure      Outcome = "failure"
)

type Result struct {
	Outcome Outcome
	Reason  string
	Context string
}

// Encode writes the single cross-platform hook protocol: SessionStart
// context goes to stdout as plain text; Stop drift emits a block decision.
// Everything else is silent on stdout (failure warnings are the CLI's
// stderr concern).
func Encode(event string, result Result, out io.Writer) error {
	switch event {
	case "SessionStart":
		if result.Context == "" {
			return nil
		}
		_, err := io.WriteString(out, result.Context)
		return err
	case "Stop":
		if result.Outcome != OutcomeDrift {
			return nil
		}
		return json.NewEncoder(out).Encode(map[string]string{
			"decision": "block",
			"reason":   result.Reason,
		})
	}
	return nil
}
