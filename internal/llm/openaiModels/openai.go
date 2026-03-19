package openaiModels

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/llm"
	"github.com/akolanti/GoAPI/pkg/logger_i"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
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

	logger.With("traceId", ctx.Value("traceId"))

	var contextBuilder strings.Builder
	contextBuilder.WriteString("This is the context:\n")
	contextBuilder.WriteString(strings.Join(matches, "\n"))

	if messageHistory != nil && len(messageHistory) > 0 {
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

func closeClient(ctx context.Context, llm *llmClient) {
	<-ctx.Done()
	logger.Info("Closing OpenAI client")
	llm.client = nil
	llm.modelName = ""
	llm.prompt = ""
}
