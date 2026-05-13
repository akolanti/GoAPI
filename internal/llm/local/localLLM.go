package local

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/llm"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

var logger *logger_i.Logger
var localClient *llmClient
var once sync.Once

type llmClient struct {
	httpClient *http.Client
	modelName  string
}

func GetLocalClient(ctx context.Context, modelName string) llm.Provider {
	once.Do(func() {
		logger = logger_i.NewLogger("llm_local")
		newLocalClient(ctx, modelName)
	})

	if localClient == nil {
		return nil
	}

	return &llmClient{
		httpClient: localClient.httpClient,
		modelName:  localClient.modelName,
	}
}

func newLocalClient(ctx context.Context, modelName string) {
	client := &http.Client{
		Timeout: config.LocalLLMTimeout,
	}

	localClient = &llmClient{httpClient: client, modelName: modelName}
	logger.Info("Local LLM client created", "model", modelName)

	if err := waitForHealth(client); err != nil {
		logger.Error("llama-server health check failed - provider will retry on first request", "error", err)
	} else {
		logger.Info("llama-server is healthy")
	}

	go closeClient(ctx, localClient)
}

func waitForHealth(client *http.Client) error {
	healthClient := &http.Client{Timeout: 5 * time.Second}
	backoff := []time.Duration{
		500 * time.Millisecond,
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		15 * time.Second,
		30 * time.Second,
		60 * time.Second,
	}

	var lastErr error
	for _, wait := range backoff {
		resp, err := healthClient.Get(config.LocalLLMHealthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("health check returned %d (model may still be loading)", resp.StatusCode)
		} else {
			lastErr = fmt.Errorf("health check unreachable: %w", err)
		}
		logger.Debug("Waiting for llama-server", "retryIn", wait, "error", lastErr)
		time.Sleep(wait)
	}
	return fmt.Errorf("llama-server not ready after %v: %w", config.LocalLLMHealthTimeout, lastErr)
}

// --- OpenAI-compatible request/response structs ---

type chatMessage struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
	Stop        []string      `json:"stop"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
	Index   int         `json:"index"`
	// finish_reason is either "stop" or "tool_calls"
	FinishReason string `json:"finish_reason"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function functionCall `json:"function"`
}

type functionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatChoiceWithTools struct {
	Message struct {
		Role      string     `json:"role"`
		Content   string     `json:"content"`
		ToolCalls []toolCall `json:"tool_calls,omitempty"`
	} `json:"message"`
	Index        int    `json:"index"`
	FinishReason string `json:"finish_reason"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
}

type chatResponseWithTools struct {
	Choices []chatChoiceWithTools `json:"choices"`
}

// --- Provider interface implementation ---

func (c *llmClient) Generate(ctx context.Context, userQuery string, matches []string, messageHistory []string) (string, error) {
	if c.httpClient == nil {
		return "", fmt.Errorf("local LLM client is nil")
	}

	log := logger.With("traceId", ctx.Value("traceId"))

	userQuery = strings.TrimSpace(userQuery) //anything to save context window should be saved
	context := strings.Join(matches, "\n")
	userPrompt := fmt.Sprintf("%s\nQ: %s", context, userQuery)

	reqBody := chatRequest{
		Model: c.modelName,
		Messages: []chatMessage{
			{Role: "system", Content: config.OfflineSystemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: config.LocalLLMTemperature,
		MaxTokens:   config.LocalLLMMaxTokens,
		Stop:        []string{},
	}

	respBody, err := c.doRequest(ctx, reqBody)
	if err != nil {
		return "", err
	}

	log.Debug("Raw LLM response", "body", string(respBody))

	var resp chatResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		log.Error("Failed to parse local LLM response", "error", err)
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from local LLM")
	}

	answer := resp.Choices[0].Message.Content
	answer = strings.ReplaceAll(answer, "</think>", "")
	answer = strings.ReplaceAll(answer, "<think>", "")
	if answer == "" && resp.Choices[0].Message.ReasoningContent != "" {
		log.Debug("LLM returned empty answer but provided reasoning content, using that as answer", "reasoningContent", resp.Choices[0].Message.ReasoningContent)
		answer, _ = doAnotherCall(ctx, c, resp.Choices[0].Message.ReasoningContent)
	}

	log.Debug("LLM answer", "content", answer)
	return strings.TrimSpace(answer), nil
}

func (c *llmClient) ChatWithTools(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Response, error) {
	// local llm does not implement tool calls.
	return &llm.Response{
		Content:    []llm.ContentBlock{{Type: llm.ContentBlockTypeText, Text: "No Tool support in local LLM"}},
		StopReason: llm.StopReasonEndTurn,
	}, nil
}

func (c *llmClient) doRequest(ctx context.Context, reqBody chatRequest) ([]byte, error) {
	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, config.LocalLLMCompletionURL, bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("local LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("local LLM returned %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func closeClient(ctx context.Context, c *llmClient) {
	<-ctx.Done()
	logger.Info("Closing local LLM client")
	c.httpClient = nil
	c.modelName = ""
}

// there is repetitive code but I will refactor this later after I get the RAG optimized too.
func doAnotherCall(ctx context.Context, c *llmClient, reasoningContent string) (string, error) {
	log := logger.With("traceId", ctx.Value("traceId"))

	reqBody := chatRequest{
		Model: c.modelName,
		Messages: []chatMessage{
			{Role: "system", Content: "this is the reasoning content from the previous call, condense it into a final answer"},
			{Role: "user", Content: reasoningContent},
		},
		Temperature: config.LocalLLMTemperature,
		MaxTokens:   config.LocalLLMMaxTokens,
		Stop:        []string{},
	}

	respBody, err := c.doRequest(ctx, reqBody)
	if err != nil {
		return "", err
	}

	log.Debug("Raw LLM response", "body", string(respBody))

	var resp chatResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		log.Error("Failed to parse local LLM response", "error", err)
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from local LLM")
	}
	answer := resp.Choices[0].Message.Content
	if answer == "" {
		return "", fmt.Errorf("no answer returned from local LLM")
	}
	return strings.TrimSpace(answer), nil
}
