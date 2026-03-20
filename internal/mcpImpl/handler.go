package mcpImpl

import (
	"context"
	"fmt"
	"time"

	"github.com/akolanti/GoAPI/internal/domain/jobModel"
	"github.com/akolanti/GoAPI/internal/job"
	"github.com/akolanti/GoAPI/internal/llm"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

const maxToolUseIterations = 10

var logHandler *logger_i.Logger
var llmProvider llm.Provider
var jobStore jobModel.JobStore
var service *job.Service

func InitMCPHandler(provider llm.Provider, svc *job.Service) {
	logHandler = logger_i.NewLogger("mcp_handler")
	llmProvider = provider
	jobStore = svc.JobStore
	service = svc
}

func HandleRequest(ctx context.Context, question string, jobId string, traceId string) {
	//save initial job as running so the polling endpoint can find it
	initialJob := jobModel.Job{
		Id:          jobId,
		TraceId:     traceId,
		JobType:     jobModel.JobTypeMCP,
		Status:      jobModel.JobStatusRunning,
		CurrentStep: jobModel.UserQueryInit,
		CreatedTime: time.Now(),
		JobPayload: jobModel.JobPayload{
			Question: question,
		},
	}
	if err := jobStore.SaveJob(ctx, initialJob); err != nil {
		logHandler.With("traceId", traceId).Error("Failed to save initial MCP job", "error", err)
		return
	}

	go func() {
		answer, err := runToolLoop(context.Background(), question, jobId)

		if err != nil {
			logHandler.With("traceId", traceId).Error("MCP tool loop error", "error", err)
			initialJob.Status = jobModel.JobStatusError
			initialJob.CurrentStep = jobModel.Error
			initialJob.EndTime = time.Now()
			initialJob.Error = jobModel.JobError{
				Code:    500,
				Message: err.Error(),
				Retry:   true,
			}
			_ = jobStore.SaveJob(context.Background(), initialJob)
			return
		}

		initialJob.Status = jobModel.JobStatusComplete
		initialJob.CurrentStep = jobModel.Complete
		initialJob.EndTime = time.Now()
		initialJob.JobPayload.Answer = answer
		_ = jobStore.SaveJob(context.Background(), initialJob)
	}()
}

// message list only lives here - nothing persisted to redis
func runToolLoop(ctx context.Context, question string, id string) (string, error) {
	if llmProvider == nil {
		return "", fmt.Errorf("LLM provider not initialised")
	}

	logHandler.With("traceId", id).Info("Handling MCP request")

	mcpTools, err := ListMCPTools(ctx)
	if err != nil {
		return "", fmt.Errorf("fetching MCP tools: %w", err)
	}

	tools := make([]llm.Tool, 0, len(mcpTools))
	for _, t := range mcpTools {
		schema, _ := t.InputSchema.(map[string]any)
		tools = append(tools, llm.Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schema,
		})
	}

	messages := []llm.Message{
		{
			Role: llm.RoleUser,
			Content: []llm.ContentBlock{
				{Type: llm.ContentBlockTypeText, Text: question},
			},
		},
	}

	//tool use loop
	for i := 0; i < maxToolUseIterations; i++ {
		resp, err := llmProvider.ChatWithTools(ctx, messages, tools)
		if err != nil {
			return "", fmt.Errorf("ChatWithTools iteration %d: %w", i, err)
		}

		if resp.StopReason == llm.StopReasonEndTurn {
			for _, block := range resp.Content {
				if block.Type == llm.ContentBlockTypeText {
					return block.Text, nil
				}
			}
			return "", fmt.Errorf("model returned end_turn but no text content")
		}

		//tool use - append assistant msg and execute tools
		messages = append(messages, llm.Message{
			Role:    llm.RoleAssistant,
			Content: resp.Content,
		})

		toolResultBlocks := make([]llm.ContentBlock, 0, len(resp.Content))
		for _, block := range resp.Content {
			if block.Type != llm.ContentBlockTypeToolUse {
				continue
			}

			logHandler.With("traceId", id).Info("Calling MCP tool", "tool", block.ToolName)
			result, toolErr := CallMCPTool(ctx, block.ToolName, block.ToolArgs)
			if toolErr != nil {
				logHandler.With("traceId", id).Error("MCP tool error", "tool", block.ToolName, "error", toolErr)
				result = fmt.Sprintf("error: %v", toolErr)
			}

			toolResultBlocks = append(toolResultBlocks, llm.ContentBlock{
				Type:       llm.ContentBlockTypeToolResult,
				ToolCallID: block.ToolCallID,
				ToolName:   block.ToolName,
				ToolResult: result,
			})
		}

		messages = append(messages, llm.Message{
			Role:    llm.RoleUser,
			Content: toolResultBlocks,
		})
	}

	return "", fmt.Errorf("exceeded maximum tool-use iterations (%d)", maxToolUseIterations)
}
