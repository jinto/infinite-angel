package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/jinto/ina/daemon"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	s := server.NewMCPServer("ina", "1.0.0",
		server.WithToolCapabilities(true),
	)

	addReportProgress(s)
	addMarkBlocked(s)
	addCheckAgents(s)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("mcp server error: %v", err)
	}
}

type progressReport struct {
	InProgress string `json:"in_progress"`
	Completed  string `json:"completed"`
	Remaining  string `json:"remaining"`
	Context    string `json:"context"`
}

func addReportProgress(s *server.MCPServer) {
	tool := mcp.NewTool("ina_report_progress",
		mcp.WithDescription("Report task progress to the ina watchdog daemon. Call this periodically to keep the daemon informed of your work."),
		mcp.WithString("completed", mcp.Description("Comma-separated list of completed items")),
		mcp.WithString("in_progress", mcp.Description("What you're currently working on"), mcp.Required()),
		mcp.WithString("remaining", mcp.Description("Comma-separated list of remaining items")),
		mcp.WithString("context", mcp.Description("Context for another agent to continue if you crash")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		inProgress, _ := args["in_progress"].(string)
		completed, _ := args["completed"].(string)
		remaining, _ := args["remaining"].(string)
		ctxStr, _ := args["context"].(string)

		return callDaemon(daemon.ActionProgress, progressReport{
			InProgress: inProgress,
			Completed:  completed,
			Remaining:  remaining,
			Context:    ctxStr,
		})
	})
}

func addMarkBlocked(s *server.MCPServer) {
	tool := mcp.NewTool("ina_mark_blocked",
		mcp.WithDescription("Tell the ina daemon that you are blocked and need human intervention."),
		mcp.WithString("reason", mcp.Description("Why you are blocked"), mcp.Required()),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		reason, _ := req.GetArguments()["reason"].(string)

		return callDaemon(daemon.ActionBlocked, struct {
			Reason string `json:"reason"`
		}{Reason: reason})
	})
}

func addCheckAgents(s *server.MCPServer) {
	tool := mcp.NewTool("ina_check_agents",
		mcp.WithDescription("Check the status of all agents tracked by the ina watchdog daemon."),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		resp, err := sendToDaemon(daemon.ActionStatus, nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("daemon error: %v", err)), nil
		}

		var data any
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("Tracked agents:\n%s", resp.Data)), nil
		}
		indented, _ := json.MarshalIndent(data, "", "  ")
		return mcp.NewToolResultText(fmt.Sprintf("Tracked agents:\n%s", indented)), nil
	})
}

func callDaemon(action string, payload any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
	}
	resp, err := daemon.SendCommand(daemon.Command{Action: action, Data: data})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("daemon error: %v", err)), nil
	}
	return mcp.NewToolResultText(resp.Message), nil
}

func sendToDaemon(action string, data json.RawMessage) (*daemon.Response, error) {
	return daemon.SendCommand(daemon.Command{Action: action, Data: data})
}
