package gemini

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/llm"
	"github.com/akolanti/GoAPI/pkg/logger_i"
	"google.golang.org/genai"
)

type llmClient struct {
	client    *genai.Client
	modelName string
	prompt    string
}

var logger *logger_i.Logger
var geminiClient *llmClient
var once sync.Once

func GetGeminiClient(ctx context.Context, modelName string, apikey string) llm.Provider {
	once.Do(func() {
		logger = logger_i.NewLogger("llm_gemini")
		newGeminiClient(ctx, modelName, apikey)
	})

	if geminiClient == nil {
		return nil
	}
	return &llmClient{client: geminiClient.client, modelName: geminiClient.modelName, prompt: geminiClient.prompt}
}

func newGeminiClient(ctx context.Context, apikey string, modelName string) {

	c, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apikey})
	if err != nil {
		logger.Error("Error creating Gemini client:", "error", err)
	}
	if c != nil {
		geminiClient = &llmClient{client: c, modelName: modelName, prompt: config.LLMPrompt}
		logger.Debug("Gemini ", modelName, " client created")
		logger.Info("Gemini client created")
		go closeClient(ctx, geminiClient)
	}

}

func (c *llmClient) Generate(ctx context.Context, userQuery string, matches []string, messageHistory []string) (string, error) {
	logger.With("traceId", ctx.Value("traceId"))
	systemInstruction := &genai.Content{
		Parts: []*genai.Part{
			{Text: config.ModelContext},
		},
	}

	contextText := "This is the context :"

	if messageHistory != nil && len(messageHistory) > 0 {
		contextText = contextText + "\n This is Message History :" +
			" Question stands for the user question and the answer stands for the answer you gave, sources are the source for answer \n"
		contextText = contextText + strings.Join(messageHistory, "\n")

	}
	contextText = strings.Join(matches, "\n")
	userPrompt := fmt.Sprintf("Context:\n%s\n\nUser Question: %s", contextText, userQuery)

	contentConfig := &genai.GenerateContentConfig{
		SystemInstruction: systemInstruction,
	}

	result, _ := c.client.Models.GenerateContent(
		ctx,
		c.modelName,
		genai.Text(userPrompt),
		contentConfig,
	)
	return result.Text(), nil
}

func (c *llmClient) ChatWithTools(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Response, error) {
	if c.client == nil {
		return nil, fmt.Errorf("gemini client is nil")
	}

	logger.With("traceId", ctx.Value("traceId"))

	funcDecls := make([]*genai.FunctionDeclaration, 0, len(tools))
	for _, t := range tools {
		funcDecls = append(funcDecls, &genai.FunctionDeclaration{
			Name:                 t.Name,
			Description:          t.Description,
			ParametersJsonSchema: t.InputSchema,
		})
	}

	genaiConfig := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: config.ModelContext}},
		},
		Tools: []*genai.Tool{
			{FunctionDeclarations: funcDecls},
		},
	}

	contents := toGeminiContents(messages)

	result, err := c.client.Models.GenerateContent(ctx, c.modelName, contents, genaiConfig)
	if err != nil {
		logger.Error("Error calling Gemini ChatWithTools:", "error", err)
		return nil, err
	}

	return fromGeminiResponse(result), nil
}

func toGeminiContents(messages []llm.Message) []*genai.Content {
	contents := make([]*genai.Content, 0, len(messages))
	for _, msg := range messages {
		parts := make([]*genai.Part, 0, len(msg.Content))
		for _, block := range msg.Content {
			switch block.Type {
			case llm.ContentBlockTypeText:
				parts = append(parts, genai.NewPartFromText(block.Text))
			case llm.ContentBlockTypeToolUse:
				parts = append(parts, genai.NewPartFromFunctionCall(block.ToolName, block.ToolArgs))
			case llm.ContentBlockTypeToolResult:
				parts = append(parts, genai.NewPartFromFunctionResponse(block.ToolName, map[string]any{
					"result": block.ToolResult,
				}))
			}
		}

		role := genai.RoleUser
		if msg.Role == llm.RoleAssistant {
			role = genai.RoleModel
		}
		contents = append(contents, genai.NewContentFromParts(parts, genai.Role(role)))
	}
	return contents
}

func fromGeminiResponse(result *genai.GenerateContentResponse) *llm.Response {
	if len(result.Candidates) == 0 {
		return &llm.Response{StopReason: llm.StopReasonEndTurn}
	}

	candidate := result.Candidates[0]
	if candidate.Content == nil {
		return &llm.Response{StopReason: llm.StopReasonEndTurn}
	}

	blocks := make([]llm.ContentBlock, 0, len(candidate.Content.Parts))
	hasToolCall := false

	for _, part := range candidate.Content.Parts {
		if part.FunctionCall != nil {
			hasToolCall = true
			callID := part.FunctionCall.ID
			if callID == "" {
				callID = part.FunctionCall.Name
			}
			blocks = append(blocks, llm.ContentBlock{
				Type:       llm.ContentBlockTypeToolUse,
				ToolCallID: callID,
				ToolName:   part.FunctionCall.Name,
				ToolArgs:   part.FunctionCall.Args,
			})
		} else if part.Text != "" {
			blocks = append(blocks, llm.ContentBlock{
				Type: llm.ContentBlockTypeText,
				Text: part.Text,
			})
		}
	}

	stopReason := llm.StopReasonEndTurn
	if hasToolCall {
		stopReason = llm.StopReasonToolUse
	}

	return &llm.Response{Content: blocks, StopReason: stopReason}
}

func closeClient(ctx context.Context, llm *llmClient) {
	<-ctx.Done()
	logger.Info("Closing Gemini client")
	llm.client = nil
	llm.modelName = ""
	llm.prompt = ""
}
