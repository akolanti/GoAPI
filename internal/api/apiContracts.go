package api

import "time"

type JobExternalStatus string

const (
	JobStatusError JobExternalStatus = "Error"
)

type JobResponse struct {
	Id        string            `json:"id" example:"job_cz109"`
	ChatId    string            `json:"chat_id" example:"chat_550"`
	Result    Result            `json:"result"`
	Error     *JobOutgoingError `json:"error,omitempty"`
	StartTime time.Time         `json:"start_time"`
	EndTime   time.Time         `json:"end_time,omitempty"`
}

type JobOutgoingError struct {
	Code    int    `json:"code" example:"400"`
	Message string `json:"message" example:"Job not found"`
	Retry   bool   `json:"can_retry" example:"false"`
}

type RAGResponse struct {
	Question string   `json:"question"`
	Answer   string   `json:"answer"`
	Sources  []string `json:"sources"`
}

type Result struct {
	Status              string       `json:"status"`
	RAGExternalResponse *RAGResponse `json:"rag_response,omitempty"`
}

type InitJobResponse struct {
	Id        string `json:"id"`
	StatusURL string `json:"status_url"`
}

// requests---------------------

type ChatRequest struct {
	Message string `json:"message" validate:"required" `
	ChatID  string `json:"chatID,omitempty" `
}
type JobStatusRequest struct {
	JobId string `json:"job_id" validate:"required"`
}

type IngestDocumentRequest struct {
	DocumentName string `json:"document_name" validate:"required"`
}
