package rag

import (
	"context"
	"net/http"
	"time"

	"github.com/akolanti/GoAPI/internal/domain/jobModel"
	"github.com/akolanti/GoAPI/internal/metrics"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

func returnOutput(job jobModel.Job, ans string) jobModel.Job {
	job.JobPayload.Answer = ans
	job.CurrentStep = jobModel.Complete
	return job
}

func logOutput(job jobModel.Job, status jobModel.InternalStatus, log *logger_i.Logger) jobModel.Job {
	job.CurrentStep = status
	log.Debug("ProcessRequest", "Current Status", job.CurrentStep)
	return job
}

func (s *service) jobError(job jobModel.Job, err error, message string, canRetry bool) jobModel.Job {
	s.logger.Error(message, "error", err)

	job.Error = jobModel.JobError{
		Code:    http.StatusInternalServerError,
		Message: "Internal Server Error",
		Retry:   canRetry,
	}
	job.Status = jobModel.JobStatusError
	return job
}
func (s *service) executeEmbeddingStep(ctx context.Context, log *logger_i.Logger, job *jobModel.Job) ([]float32, error) {
	*job = logOutput(*job, jobModel.EmbeddingAPICall, log)

	start := time.Now()
	defer func() { metrics.CaptureExecutionMetrics("embedding", time.Since(start)) }()

	return s.embedder.GetEmbedding(ctx, job.JobPayload.Question)
}

func (s *service) executeCacheCheckStep(ctx context.Context, log *logger_i.Logger, job *jobModel.Job, emb []float32) (string, bool) {
	*job = logOutput(*job, jobModel.CacheCall, log)

	start := time.Now()
	defer func() { metrics.CaptureExecutionMetrics("cache_lookup", time.Since(start)) }()

	ans, found, _ := s.vectorDB.GetCachedAnswer(ctx, emb)
	return ans, found
}

func (s *service) executeVectorSearchStep(ctx context.Context, log *logger_i.Logger, job *jobModel.Job, emb []float32) ([]string, error) {
	*job = logOutput(*job, jobModel.VectorDBCall, log)

	start := time.Now()
	defer func() { metrics.CaptureExecutionMetrics("vector_search", time.Since(start)) }()

	matches, metaData, err := s.vectorDB.Search(ctx, emb)
	job.JobPayload.Sources = metaData
	return matches, err
}

func (s *service) executeLLMStep(ctx context.Context, log *logger_i.Logger, job *jobModel.Job, matches []string, history []string) (string, error) {
	*job = logOutput(*job, jobModel.LLMCall, log)

	start := time.Now()
	defer func() { metrics.CaptureExecutionMetrics("llm_generation", time.Since(start)) }()

	return s.llmProvider.Generate(ctx, job.JobPayload.Question, matches, history)
}
