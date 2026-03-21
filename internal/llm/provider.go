package llm

import "context"

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type ContentBlockType string

const (
	ContentBlockTypeText       ContentBlockType = "text"
	ContentBlockTypeToolUse    ContentBlockType = "tool_use"
	ContentBlockTypeToolResult ContentBlockType = "tool_result"
)

type ContentBlock struct {
	Type ContentBlockType

	Text string

	//tool use
	ToolCallID string
	ToolName   string
	ToolArgs   map[string]any

	//tool result
	ToolResult string

	//raw fields because gemini needs thought signature bruh
	RawField any
}

type Message struct {
	Role    Role
	Content []ContentBlock
}

type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any //json schema
}

type StopReason string

const (
	StopReasonEndTurn StopReason = "end_turn"
	StopReasonToolUse StopReason = "tool_use"
)

type Response struct {
	Content    []ContentBlock
	StopReason StopReason
}

type Provider interface {
	Generate(ctx context.Context, query string, matches []string, messageHistory []string) (string, error)
	ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*Response, error)
}
