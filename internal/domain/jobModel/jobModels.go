package jobModel

import (
	"context"
	"time"
)

type JobStatus string
type InternalStatus string

type JobType string

const (
	JobStatusQueued   JobStatus = "QUEUED"
	JobStatusRunning  JobStatus = "RUNNING"
	JobStatusComplete JobStatus = "COMPLETE"
	JobStatusError    JobStatus = "Error"

	UserQueryInit    InternalStatus = "Init"
	CacheCall        InternalStatus = "CacheCall"
	RAGCall          InternalStatus = "RAG"
	LLMCall          InternalStatus = "LLM"
	VectorDBCall     InternalStatus = "VectorDB"
	EmbeddingAPICall InternalStatus = "EmbeddingAPI"
	RedisCall        InternalStatus = "Redis"

	IngestInit       InternalStatus = "IngestInit"
	IngestProcessing InternalStatus = "IngestProcessing"
	Error            InternalStatus = "Error"

	Complete InternalStatus = "Complete"

	JobTypeQuery  JobType = "Query"
	JobTypeIngest JobType = "Ingest"
)

type Job struct {
	Id          string         `json:"id"`
	ChatId      string         `json:"chat_id"`
	TraceId     string         `json:"trace_id"`
	JobType     JobType        `json:"job_type"`
	JobPayload  JobPayload     `json:"job_payload"`
	Error       JobError       `json:"error,omitempty"`
	CreatedTime time.Time      `json:"created_time"`
	EndTime     time.Time      `json:"end_time,omitempty"`
	Status      JobStatus      `json:"status"`
	CurrentStep InternalStatus `json:"current_step"`
}

type JobError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Retry   bool   `json:"retry"`
}

type JobPayload struct {
	Question string   `json:"question,omitempty"`
	Answer   string   `json:"answer,omitempty"`
	Sources  []string `json:"sources,omitempty"`

	IngestFileName string `json:"ingest_file_name,omitempty"`
	IngestURL      string `json:"ingest_url,omitempty"`
}

type JobStore interface {
	GetJob(ctx context.Context, jobId string) (Job, bool)
	SaveJob(ctx context.Context, job Job) error
	DeleteJob(ctx context.Context, jobID string)
}

type MessageStore interface {
	ValidateChatId(ctx context.Context, id string) bool
	TrySaveChat(ctx context.Context, id string, JobPayload JobPayload) error
	InitNewChat(ctx context.Context, id string) error
	GetMessageHistory(ctx context.Context, chatId string) (error, []string)
}
