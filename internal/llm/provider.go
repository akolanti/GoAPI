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

// ContentBlock is one typed fragment inside a Message.
type ContentBlock struct {
	Type ContentBlockType

	// text block
	Text string

	// tool_use block (assistant requesting a tool call)
	ToolCallID string
	ToolName   string
	ToolArgs   map[string]any

	// tool_result block (user returning the tool result)
	ToolResult string
}

// Message is a provider-agnostic chat message.
type Message struct {
	Role    Role
	Content []ContentBlock
}

// Tool describes a callable tool the model may invoke.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any // JSON Schema object
}

type StopReason string

const (
	StopReasonEndTurn StopReason = "end_turn"
	StopReasonToolUse StopReason = "tool_use"
)

// Response is the model's reply from a ChatWithTools call.
type Response struct {
	Content    []ContentBlock
	StopReason StopReason
}

type Provider interface {
	Generate(ctx context.Context, query string, matches []string, messageHistory []string) (string, error)
	ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*Response, error)
}


