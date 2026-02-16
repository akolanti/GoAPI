package gemini

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/rag/llm"
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

	// 2. Prepare the user's prompt (Query + Context)
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

	//return "simulated output", nil
}

func closeClient(ctx context.Context, llm *llmClient) {
	<-ctx.Done()
	logger.Info("Closing Gemini client")
	llm.client = nil
	llm.modelName = ""
	llm.prompt = ""
}
