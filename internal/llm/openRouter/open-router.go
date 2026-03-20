package openRouter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/llm"
	"github.com/akolanti/GoAPI/pkg/logger_i"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
)

const openRouterBaseURL = "https://openrouter.ai/api/v1"

type llmClient struct {
	client    *openai.Client
	modelName string
	prompt    string
}

var logger *logger_i.Logger
var openRouterClient *llmClient
var once sync.Once

func GetOpenRouterClient(ctx context.Context, modelName string, apikey string) llm.Provider {
	once.Do(func() {
		logger = logger_i.NewLogger("llm_openrouter")
		newOpenRouterClient(ctx, modelName, apikey)
	})

	if openRouterClient == nil {
		return nil
	}

	return &llmClient{
		client:    openRouterClient.client,
		modelName: openRouterClient.modelName,
		prompt:    openRouterClient.prompt,
	}
}

func newOpenRouterClient(ctx context.Context, modelName string, apikey string) {
	c := openai.NewClient(
		option.WithAPIKey(apikey),
		option.WithBaseURL(openRouterBaseURL),
	)

	openRouterClient = &llmClient{client: &c, modelName: modelName, prompt: config.LLMPrompt}
	logger.Debug("OpenRouter ", modelName, " client created")
	logger.Info("OpenRouter client created")
	go closeClient(ctx, openRouterClient)
}

func (c *llmClient) Generate(ctx context.Context, userQuery string, matches []string, messageHistory []string) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("openrouter client is nil")
	}

	logger.With("traceId", ctx.Value("traceId"))

	var contextBuilder strings.Builder
	contextBuilder.WriteString("This is the context:\n")
	contextBuilder.WriteString(strings.Join(matches, "\n"))

	if len(messageHistory) > 0 {
		contextBuilder.WriteString("\n\nThis is Message History:\n")
		contextBuilder.WriteString("Question stands for the user question and the answer stands for the answer you gave, sources are the source for answer.\n")
		contextBuilder.WriteString(strings.Join(messageHistory, "\n"))
	}

	userPrompt := fmt.Sprintf("Context:\n%s\n\nUser Question: %s", contextBuilder.String(), userQuery)

	params := openai.ChatCompletionNewParams{
		Model: c.modelName,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(config.ModelContext),
			openai.UserMessage(userPrompt),
		},
	}

	completion, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		logger.Error("Error generating content from OpenRouter:", "error", err)
		return "", err
	}

	if len(completion.Choices) > 0 {
		return completion.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no content returned from OpenRouter")
}

func (c *llmClient) ChatWithTools(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Response, error) {
	if c.client == nil {
		return nil, fmt.Errorf("openrouter client is nil")
	}

	logger.With("traceId", ctx.Value("traceId"))

	oaiTools := make([]openai.ChatCompletionToolParam, 0, len(tools))
	for _, t := range tools {
		oaiTools = append(oaiTools, openai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        t.Name,
				Description: openai.String(t.Description),
				Parameters:  openai.FunctionParameters(t.InputSchema),
			},
		})
	}

	oaiMessages := toOpenRouterMessages(messages)

	completion, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    c.modelName,
		Messages: oaiMessages,
		Tools:    oaiTools,
	})
	if err != nil {
		logger.Error("Error calling OpenRouter ChatWithTools:", "error", err)
		return nil, err
	}

	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from OpenRouter")
	}

	return fromOpenRouterResponse(completion.Choices[0]), nil
}

func toOpenRouterMessages(messages []llm.Message) []openai.ChatCompletionMessageParamUnion {
	result := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case llm.RoleUser:
			for _, block := range msg.Content {
				switch block.Type {
				case llm.ContentBlockTypeText:
					result = append(result, openai.UserMessage(block.Text))
				case llm.ContentBlockTypeToolResult:
					result = append(result, openai.ToolMessage(block.ToolResult, block.ToolCallID))
				}
			}
		case llm.RoleAssistant:
			var toolCalls []openai.ChatCompletionMessageToolCallParam
			var text string
			for _, block := range msg.Content {
				switch block.Type {
				case llm.ContentBlockTypeText:
					text = block.Text
				case llm.ContentBlockTypeToolUse:
					argsJSON, _ := json.Marshal(block.ToolArgs)
					toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallParam{
						ID: block.ToolCallID,
						Function: openai.ChatCompletionMessageToolCallFunctionParam{
							Name:      block.ToolName,
							Arguments: string(argsJSON),
						},
					})
				}
			}
			if len(toolCalls) > 0 {
				result = append(result, openai.ChatCompletionMessageParamUnion{
					OfAssistant: &openai.ChatCompletionAssistantMessageParam{
						ToolCalls: toolCalls,
					},
				})
			} else {
				result = append(result, openai.AssistantMessage(text))
			}
		}
	}
	return result
}

func fromOpenRouterResponse(choice openai.ChatCompletionChoice) *llm.Response {
	if choice.FinishReason == "tool_calls" {
		blocks := make([]llm.ContentBlock, 0, len(choice.Message.ToolCalls))
		for _, tc := range choice.Message.ToolCalls {
			var args map[string]any
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
			blocks = append(blocks, llm.ContentBlock{
				Type:       llm.ContentBlockTypeToolUse,
				ToolCallID: tc.ID,
				ToolName:   tc.Function.Name,
				ToolArgs:   args,
			})
		}
		return &llm.Response{Content: blocks, StopReason: llm.StopReasonToolUse}
	}

	return &llm.Response{
		Content:    []llm.ContentBlock{{Type: llm.ContentBlockTypeText, Text: choice.Message.Content}},
		StopReason: llm.StopReasonEndTurn,
	}
}

func closeClient(ctx context.Context, llm *llmClient) {
	<-ctx.Done()
	logger.Info("Closing OpenRouter client")
	llm.client = nil
	llm.modelName = ""
	llm.prompt = ""
}
