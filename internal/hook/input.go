package hook

import (
	"encoding/json"
	"errors"
)

type Input struct {
	SessionID      string `json:"session_id"`
	CWD            string `json:"cwd"`
	EventName      string `json:"hook_event_name"`
	Source         string `json:"source,omitempty"`
	StopHookActive bool   `json:"stop_hook_active,omitempty"`
}

func Decode(data []byte) (Input, error) {
	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		return Input{}, err
	}
	if input.SessionID == "" || input.CWD == "" || input.EventName == "" {
		return Input{}, errors.New("session_id, cwd, and hook_event_name are required")
	}
	switch input.EventName {
	case "SessionStart", "Stop":
		return input, nil
	default:
		return Input{}, errors.New("unsupported hook event")
	}
}
