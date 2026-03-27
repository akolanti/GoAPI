package mcpImpl

import (
	"context"
	"fmt"
	"sync"

	"github.com/akolanti/GoAPI/pkg/logger_i"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var logClient *logger_i.Logger
var mcpSession *mcp.ClientSession
var clientOnce sync.Once
var mcpTools []*mcp.Tool

//ok, so this is how it works

//in mcp there are 3 moving parts
//host application is this api
//the client is this mcp client implementation
// it connects to the server and keeps the session alive
//the server is the mcp server implementation, it defines the tools and the tool handlers

//First we create a client and a server.

//the client is responsible for
//first starting a server
//connecting to it and listening to it.
//it also inits and keeps a session open
//the llm uses the client to talk to any mcp server tool
//this client is the uniform interface the client can use

// the server is started as a child process for/by the client itself
// next, it says that hey, this is what I tools I have
//if you call it this tool, you will get the response in the following format
// this format is predefined when we define the tool definition

// call once at startup - spawns the mcp server binary
func InitMCPClient(ctx context.Context, transport *mcp.InMemoryTransport) {

	logClient = logger_i.NewLogger("mcp_client")
	session, err := connectMCPServer(ctx, transport)
	if err != nil {
		logClient.Error("Failed to connect to MCP server", "error", err)
		return
	}
	mcpSession = session
	logClient.Info("MCP client connected")
	er := InitMCPTools(ctx)
	if er != nil {
		logClient.With("error", er).Error("Error initialising MCP tools")
	}
	go func() {
		<-ctx.Done()
		_ = mcpSession.Close()
		logClient.Info("MCP client closed")
	}()
}

func connectMCPServer(ctx context.Context, transport *mcp.InMemoryTransport) (*mcp.ClientSession, error) {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "GoAPI MCP Client",
		Version: "1.0.0",
	}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		logClient.With("error", err).Error("Error connecting to MCP server")
		return nil, fmt.Errorf("connecting to MCP server: %w", err)
	}
	return session, nil
}

func InitMCPTools(ctx context.Context) error {

	if mcpSession == nil {
		return fmt.Errorf("MCP client not initialised")
	}

	result, err := mcpSession.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return fmt.Errorf("listing MCP tools: %w", err)
	}
	mcpTools = result.Tools
	return nil
}

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
	logClient.With("tool", name).Debug("MCP tool call successful")
	var sb []byte
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			sb = append(sb, tc.Text...)
		}
	}
	return string(sb), nil
}
