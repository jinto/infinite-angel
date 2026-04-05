package daemon

import "encoding/json"

const (
	ActionStatus   = "status"
	ActionLaunch   = "launch"
	ActionRestart  = "restart"
	ActionStop     = "stop"
	ActionProgress = "progress"
	ActionBlocked  = "blocked"
)

type Command struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data,omitempty"`
}

type Response struct {
	OK      bool            `json:"ok"`
	Message string          `json:"message,omitempty"`
	Error   string          `json:"error,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}
