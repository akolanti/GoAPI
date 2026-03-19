package mcpImpl

import (
	"context"

	"github.com/akolanti/GoAPI/internal/ragBridge"
	"github.com/akolanti/GoAPI/pkg/logger_i"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var logMCP *logger_i.Logger

func InitMCP() {
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

func RAGQuery(ctx context.Context, req *mcp.CallToolRequest, in ProcessQueryStruct) (*mcp.CallToolResult, QueryResponseStruct, error) {
	logMCP.With("traceId", in.Id).Info("Received RAG Query tool call")

	res := ragBridge.ProcessQuery(ctx, in.Query, in.Id)

	return nil, QueryResponseStruct{
		Id:       res.Id,
		Query:    res.Query,
		Response: res.Response,
		Sources:  res.Sources,
		Status:   res.Status,
		Err:      res.Err,
		DoRetry:  res.DoRetry,
	}, res.Err

}

type ProcessQueryStruct struct {
	Query string `json:"Query"`
	Id    string `json:"id"`
}

type QueryResponseStruct struct {
	Id       string   `json:"id"`
	Query    string   `json:"query"`
	Response string   `json:"response"`
	Sources  []string `json:"sources"`
	Status   string   `json:"status"`
	Err      error    `json:"err,omitempty"`
	DoRetry  bool     `json:"do_retry"`
}
