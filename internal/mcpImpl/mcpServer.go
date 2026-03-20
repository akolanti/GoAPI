package mcpImpl

import (
	"context"

	"github.com/akolanti/GoAPI/pkg/logger_i"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var logMCP *logger_i.Logger

func InitMCPServer() {
	logMCP = logger_i.NewLogger("MCP:")

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "GoAPI MCP Server",
		Version: "1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "RAG Query",
		Description: "This tool takes a user query and returns a response based on the RAG system. The response includes the answer to the query, the sources used to generate the answer, and the status of the query processing.",
	}, RAGQuery)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		logMCP.With("error", err).Error("Failed to start MCP server")
	}
}

func RAGQuery(ctx context.Context, req *mcp.CallToolRequest, in ProcessQueryStruct) (*mcp.CallToolResult, QueryResult, error) {
	logMCP.With("traceId", in.Id).Info("Received RAG Query tool call")

	res := ProcessQuery(ctx, in.Query, in.Id)
	return nil, res, res.Err
}

type ProcessQueryStruct struct {
	Query string `json:"Query"`
	Id    string `json:"id"`
}
