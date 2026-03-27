package openaiModels

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

type llmClient struct {
	client    *openai.Client
	modelName string
	prompt    string
}

var logger *logger_i.Logger
var openAIClient *llmClient
var once sync.Once

func GetOpenAIClient(ctx context.Context, modelName string, apikey string) llm.Provider {
	once.Do(func() {
		logger = logger_i.NewLogger("llm_openai")
		newOpenAIClient(ctx, modelName, apikey)
	})

	if openAIClient == nil {
		return nil
	}

	return &llmClient{
		client:    openAIClient.client,
		modelName: openAIClient.modelName,
		prompt:    openAIClient.prompt,
	}
}

func newOpenAIClient(ctx context.Context, modelName string, apikey string) {

	c := openai.NewClient(
		option.WithAPIKey(apikey),
	)

	openAIClient = &llmClient{client: &c, modelName: modelName, prompt: config.LLMPrompt}
	logger.Debug("OpenAI ", modelName, " client created")
	logger.Info("OpenAI client created")
	go closeClient(ctx, openAIClient)

}

func (c *llmClient) Generate(ctx context.Context, userQuery string, matches []string, messageHistory []string) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("openai client is nil")
	}

	log := logger.With("traceId", ctx.Value("traceId"))
	_ = log

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
		logger.Error("Error generating content from OpenAI:", "error", err)
		return "", err
	}

	if len(completion.Choices) > 0 {
		return completion.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no content returned from OpenAI")
}

func (c *llmClient) ChatWithTools(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Response, error) {
	if c.client == nil {
		return nil, fmt.Errorf("openai client is nil")
	}

	//add logging later
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

	oaiMessages := append(
		[]openai.ChatCompletionMessageParamUnion{openai.SystemMessage(config.ModelContext)},
		toOpenAIMessages(messages)...,
	)

	completion, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    c.modelName,
		Messages: oaiMessages,
		Tools:    oaiTools,
	})
	if err != nil {
		logger.Error("Error calling OpenAI ChatWithTools:", "error", err)
		return nil, err
	}

	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from OpenAI")
	}

	return fromOpenAIResponse(completion.Choices[0]), nil
}

func toOpenAIMessages(messages []llm.Message) []openai.ChatCompletionMessageParamUnion {
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
				assistantMsg := &openai.ChatCompletionAssistantMessageParam{
					ToolCalls: toolCalls,
				}
				if text != "" {
					assistantMsg.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: openai.String(text),
					}
				}
				result = append(result, openai.ChatCompletionMessageParamUnion{
					OfAssistant: assistantMsg,
				})
			} else {
				result = append(result, openai.AssistantMessage(text))
			}
		}
	}
	return result
}

func fromOpenAIResponse(choice openai.ChatCompletionChoice) *llm.Response {
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
	logger.Info("Closing OpenAI client")
	llm.client = nil
	llm.modelName = ""
	llm.prompt = ""
}
