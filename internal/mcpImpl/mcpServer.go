package mcpImpl

import (
	"context"

	"github.com/akolanti/GoAPI/internal/adapter/utils"
	"github.com/akolanti/GoAPI/pkg/logger_i"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var logMCP *logger_i.Logger

func InitMCPServer(ctx context.Context, transport *mcp.InMemoryTransport) {
	logMCP = logger_i.NewLogger("MCP:")

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "GoAPI MCP Server",
		Version: "1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_knowledge_base",
		Description: "This tool takes a user query and returns a response based on the RAG system. The response includes the answer to the query, the sources used to generate the answer, and the status of the query processing.",
	}, search_knowledge_base)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_system_message",
		Description: "Looks up a system message by its code from the external system messages API. Returns the message code, language, and description. Example codes but do not use these codes directly, these are just examples (ATTACHDISP,ABORTWF). Use the input from the user query to look up the system message.",
	}, get_system_message)

	if err := server.Run(ctx, transport); err != nil {
		logMCP.With("error", err).Error("Failed to start MCP server")
	}
	logMCP.Info("MCP server stopped")
}

// renamed this so model can understand easily. I might need to rename it even more. change the struct too.
func search_knowledge_base(ctx context.Context, req *mcp.CallToolRequest, in ProcessQueryStruct) (*mcp.CallToolResult, QueryResult, error) {
	id := utils.GetNewUUID()
	logMCP.With("traceId", id).Info("Received RAG Query tool call")

	res := ProcessQuery(ctx, in.Query, id)
	return nil, res, res.Err
}

type ProcessQueryStruct struct {
	Query string `json:"Query"`
}
