package factory

import (
	"context"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/llm"
	"github.com/akolanti/GoAPI/internal/llm/claude"
	"github.com/akolanti/GoAPI/internal/llm/gemini"
	"github.com/akolanti/GoAPI/internal/llm/openRouter"
	"github.com/akolanti/GoAPI/internal/llm/openaiModels"
)

func NewProvider(ctx context.Context) llm.Provider {
	switch config.LLMProvider {
	case "gemini":
		return gemini.GetGeminiClient(ctx, config.LLMModelName, config.LLMAPIKey)
	case "claude":
		return claude.GetClaudeClient(ctx, config.LLMModelName, config.LLMAPIKey)
	case "openai":
		return openaiModels.GetOpenAIClient(ctx, config.LLMModelName, config.LLMAPIKey)
	case "openrouter":
		return openRouter.GetOpenRouterClient(ctx, config.LLMModelName, config.LLMAPIKey)
	default:
		return nil
	}
}
