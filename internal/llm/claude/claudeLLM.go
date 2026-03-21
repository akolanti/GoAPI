package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/llm"
	"github.com/akolanti/GoAPI/pkg/logger_i"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type llmClient struct {
	client    *anthropic.Client
	modelName string
	prompt    string
}

var logger *logger_i.Logger
var claudeClient *llmClient
var once sync.Once

func GetClaudeClient(ctx context.Context, modelName string, apikey string) llm.Provider {
	once.Do(func() {
		logger = logger_i.NewLogger("llm_claude")
		newClaudeClient(ctx, modelName, apikey)
	})

	if claudeClient == nil {
		return nil
	}

	return &llmClient{
		client:    claudeClient.client,
		modelName: claudeClient.modelName,
		prompt:    claudeClient.prompt,
	}
}

func newClaudeClient(ctx context.Context, modelName string, apikey string) {
	c := anthropic.NewClient(
		option.WithAPIKey(apikey),
	)

	claudeClient = &llmClient{client: &c, modelName: modelName, prompt: config.LLMPrompt}
	logger.Debug("Claude ", modelName, " client created")
	logger.Info("Claude client created")
	go closeClient(ctx, claudeClient)
}

func (c *llmClient) Generate(ctx context.Context, userQuery string, matches []string, messageHistory []string) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("claude client is nil")
	}

	//add logging later

	var contextBuilder strings.Builder
	contextBuilder.WriteString("This is the context:\n")
	contextBuilder.WriteString(strings.Join(matches, "\n"))

	if len(messageHistory) > 0 {
		contextBuilder.WriteString("\n\nThis is Message History:\n")
		contextBuilder.WriteString("Question stands for the user question and the answer stands for the answer you gave, sources are the source for answer.\n")
		contextBuilder.WriteString(strings.Join(messageHistory, "\n"))
	}

	userPrompt := fmt.Sprintf("Context:\n%s\n\nUser Question: %s", contextBuilder.String(), userQuery)

	message, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.modelName),
		MaxTokens: 1024,
		System: []anthropic.TextBlockParam{
			{Text: config.ModelContext},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		logger.Error("Error generating content from Claude:", "error", err)
		return "", err
	}

	if len(message.Content) > 0 {
		return message.Content[0].Text, nil
	}

	return "", fmt.Errorf("no content returned from Claude")
}

func (c *llmClient) ChatWithTools(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Response, error) {
	if c.client == nil {
		return nil, fmt.Errorf("claude client is nil")
	}

	log := logger.With("traceId", ctx.Value("traceId"))
	_ = log

	anthropicTools := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		anthropicTools = append(anthropicTools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name,
				Description: anthropic.String(t.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: t.InputSchema["properties"],
				},
			},
		})
	}

	anthropicMessages := make([]anthropic.MessageParam, 0, len(messages))
	for _, msg := range messages {
		anthropicMessages = append(anthropicMessages, toAnthropicMessage(msg))
	}

	resp, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.modelName),
		MaxTokens: 4096,
		System: []anthropic.TextBlockParam{
			{Text: config.ModelContext},
		},
		Messages: anthropicMessages,
		Tools:    anthropicTools,
	})
	if err != nil {
		logger.Error("Error calling Claude ChatWithTools:", "error", err)
		return nil, err
	}

	return fromAnthropicResponse(resp), nil
}

func toAnthropicMessage(msg llm.Message) anthropic.MessageParam {
	parts := make([]anthropic.ContentBlockParamUnion, 0, len(msg.Content))
	for _, block := range msg.Content {
		switch block.Type {
		case llm.ContentBlockTypeText:
			parts = append(parts, anthropic.NewTextBlock(block.Text))
		case llm.ContentBlockTypeToolUse:
			parts = append(parts, anthropic.NewToolUseBlock(block.ToolCallID, block.ToolArgs, block.ToolName))
		case llm.ContentBlockTypeToolResult:
			parts = append(parts, anthropic.NewToolResultBlock(block.ToolCallID, block.ToolResult, false))
		}
	}

	if msg.Role == llm.RoleAssistant {
		return anthropic.NewAssistantMessage(parts...)
	}
	return anthropic.NewUserMessage(parts...)
}

func fromAnthropicResponse(resp *anthropic.Message) *llm.Response {
	blocks := make([]llm.ContentBlock, 0, len(resp.Content))
	for _, cb := range resp.Content {
		switch cb.Type {
		case "text":
			blocks = append(blocks, llm.ContentBlock{
				Type: llm.ContentBlockTypeText,
				Text: cb.Text,
			})
		case "tool_use":
			var args map[string]any
			_ = json.Unmarshal(cb.Input, &args)
			blocks = append(blocks, llm.ContentBlock{
				Type:       llm.ContentBlockTypeToolUse,
				ToolCallID: cb.ID,
				ToolName:   cb.Name,
				ToolArgs:   args,
			})
		}
	}

	stopReason := llm.StopReasonEndTurn
	if resp.StopReason == anthropic.StopReasonToolUse {
		stopReason = llm.StopReasonToolUse
	}

	return &llm.Response{Content: blocks, StopReason: stopReason}
}

func closeClient(ctx context.Context, llm *llmClient) {
	<-ctx.Done()
	logger.Info("Closing Claude client")
	llm.client = nil
	llm.modelName = ""
	llm.prompt = ""
}
