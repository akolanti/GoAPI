package config

import "os"

// runtime config - overridable via environment variables
var (
	LLMProvider  = getEnvOrDefault("LLM_PROVIDER", "openrouter") // "gemini" | "claude" | "openai" | "openrouter" | "ollama" | "local"
	LLMModelName = getEnvOrDefault("LLM_MODEL_NAME", "auto")

	OllamaBaseURL string
	OFFLINE_MODE  bool

	LocalLLMBaseURL       string
	LocalLLMHealthURL     string
	LocalLLMCompletionURL string
)

func init() {
	ollamaHost := getEnvOrDefault("OLLAMA_HOST", "localhost")
	ollamaPort := getEnvOrDefault("OLLAMA_PORT", "11434")
	base := "http://" + ollamaHost + ":" + ollamaPort
	OllamaBaseURL = base + "/v1"
	LocalLLMBaseURL = base
	LocalLLMHealthURL = base + "/health"
	LocalLLMCompletionURL = base + "/v1/chat/completions"
	OFFLINE_MODE = getEnvOrDefault("OFFLINE_MODE", "false") == "true" || LLMProvider == "ollama" || LLMProvider == "local"
}

func getEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
