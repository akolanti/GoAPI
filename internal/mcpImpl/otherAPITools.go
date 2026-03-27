package mcpImpl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type SystemMessageInput struct {
	Code string `json:"code"`
}

type systemMessagesResponse struct {
	SystemMessages []systemMessage `json:"systemMessages"`
}

type systemMessage struct {
	Code        string `json:"code"`
	Language    string `json:"language"`
	Description string `json:"description"`
}

type SystemMessageOutput struct {
	Messages []systemMessage `json:"messages"`
}

func get_system_message(ctx context.Context, req *mcp.CallToolRequest, in SystemMessageInput) (*mcp.CallToolResult, SystemMessageOutput, error) {
	logMCP.With("code", in.Code).Info("Calling system messages API")

	messages, err := callSystemMessagesAPI(ctx, in.Code)
	if err != nil {
		return nil, SystemMessageOutput{}, err
	}
	return nil, SystemMessageOutput{Messages: messages}, nil
}

func callSystemMessagesAPI(ctx context.Context, code string) ([]systemMessage, error) {
	reqURL := config.SystemMessagesAPIBaseURL + "/api/v1/staticdata/systemmessages?code=" + url.QueryEscape(code)
	logMCP.With("url", reqURL).Debug("Calling system messages API")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logMCP.With("error", err).Error("Calling system messages API")
		return nil, fmt.Errorf("calling system messages API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logMCP.With("status", resp.StatusCode).Error("System messages API returned non-OK status")
		return nil, fmt.Errorf("system messages API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logMCP.With("error", err).Error("Reading response body from system messages API")
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var apiResp systemMessagesResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		logMCP.With("error", err).Error("Unmarshaling response from system messages API")
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	return apiResp.SystemMessages, nil
}
