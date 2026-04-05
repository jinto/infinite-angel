package daemon

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/jinto/ina/config"
)

// SendCommand connects to the daemon socket, sends a command, and returns the response.
// Used by both the CLI (cmd package) and the MCP server.
func SendCommand(cmd Command) (*Response, error) {
	conn, err := net.Dial("unix", config.SocketPath())
	if err != nil {
		return nil, fmt.Errorf("daemon not running: %w", err)
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(cmd); err != nil {
		return nil, err
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("%s", resp.Error)
	}

	return &resp, nil
}
