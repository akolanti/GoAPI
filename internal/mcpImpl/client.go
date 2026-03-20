package mcpImpl

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/akolanti/GoAPI/pkg/logger_i"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var logClient *logger_i.Logger
var mcpSession *mcp.ClientSession
var clientOnce sync.Once

// InitMCPClient initialises the singleton MCP client session by spawning the
// MCP server binary at serverBinaryPath.  Call this once during startup.
func InitMCPClient(ctx context.Context, serverBinaryPath string) {
	clientOnce.Do(func() {
		logClient = logger_i.NewLogger("mcp_client")
		session, err := connectMCPServer(ctx, serverBinaryPath)
		if err != nil {
			logClient.Error("Failed to connect to MCP server", "error", err)
			return
		}
		mcpSession = session
		logClient.Info("MCP client connected")
		go func() {
			<-ctx.Done()
			_ = mcpSession.Close()
			logClient.Info("MCP client closed")
		}()
	})
}

func connectMCPServer(ctx context.Context, binaryPath string) (*mcp.ClientSession, error) {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "GoAPI MCP Client",
		Version: "1.0.0",
	}, nil)

	transport := &mcp.CommandTransport{
		Command: exec.CommandContext(ctx, binaryPath, "--mcp-server"),
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connecting to MCP server: %w", err)
	}
	return session, nil
}

// ListMCPTools returns all tools advertised by the MCP server.
func ListMCPTools(ctx context.Context) ([]*mcp.Tool, error) {
	if mcpSession == nil {
		return nil, fmt.Errorf("MCP client not initialised")
	}

	result, err := mcpSession.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, fmt.Errorf("listing MCP tools: %w", err)
	}
	return result.Tools, nil
}

// CallMCPTool calls a tool by name with the given arguments and returns the
// text content of the result.
func CallMCPTool(ctx context.Context, name string, args map[string]any) (string, error) {
	if mcpSession == nil {
		return "", fmt.Errorf("MCP client not initialised")
	}

	result, err := mcpSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return "", fmt.Errorf("calling MCP tool %q: %w", name, err)
	}
	if result.IsError {
		return "", fmt.Errorf("MCP tool %q returned an error", name)
	}

	var sb []byte
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			sb = append(sb, tc.Text...)
		}
	}
	return string(sb), nil
}
