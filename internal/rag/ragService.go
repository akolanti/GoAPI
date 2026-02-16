package rag

import (
	"context"
	"errors"
	"time"

	"github.com/akolanti/GoAPI/internal/adapter/utils"
	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/domain/jobModel"
	"github.com/akolanti/GoAPI/internal/metrics"
	"github.com/akolanti/GoAPI/internal/rag/embedding"
	"github.com/akolanti/GoAPI/internal/rag/ingest"
	"github.com/akolanti/GoAPI/internal/rag/llm"
	"github.com/akolanti/GoAPI/internal/rag/vectorDB"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

/*
ARCHITECTURE NOTE: OPAQUE INTERFACE PATTERN
---------------------------------------------------------

1. Service (Interface):
  - Real work happens down low bruh
  - This is the PUBLIC contract.
  - It defines the "behavior" (what the worker can do).
  - We expose this to keep the worker decoupled from our specific logic.

2. service (Private Struct):
  - down low stuff
  - This is the PRIVATE implementation.
  - It holds the "state" (database connections and LLM clients).
  - It is lowercase to prevent external packages from accessing our
    internal dependencies (vectorDB, llmProvider) directly.

3. Pointer Receiver (*service):
  - By defining methods on (*service), the struct IMPLICITLY satisfies
    the Service interface.
  - if it quacks like a duck, -it's a duck (Duck Typing)

4. Dependency Injection (NewService):
  - This constructor links the private struct to the public interface.
  - It allows us to swap real DBs for mocks during testing without
    changing the worker's code.
*/

// Service Worker will only call this service - it doesn't need to know the llm or the vector
type Service interface {
	ProcessRequest(ctx context.Context, job jobModel.Job, messageHistory []string) jobModel.Job
	IngestDocument(ctx context.Context, job jobModel.Job) jobModel.Job
}

type service struct {
	vectorDB    vectorDB.DataProcessor
	llmProvider llm.Provider
	embedder    embedding.Embedder
	logger      *logger_i.Logger
}

// NewService constructor
func NewService(vector vectorDB.DataProcessor, llm llm.Provider, em embedding.Embedder) Service {
	return &service{
		vectorDB:    vector,
		llmProvider: llm,
		embedder:    em,
		logger:      logger_i.NewLogger("RAG Service :"),
	}
}

func (s *service) ProcessRequest(ctx context.Context, jobt jobModel.Job, messageHistory []string) jobModel.Job {
	inMethodLogger := s.logger.With("traceId", ctx.Value(config.TRACE_ID_KEY).(string), "JobId", jobt.Id)

	processContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	jobt.CurrentStep = jobModel.RAGCall

	// Embedding
	embeddingStep, err := s.executeEmbeddingStep(processContext, inMethodLogger, &jobt)
	if err != nil {
		return s.jobError(jobt, err, "EMBEDDING_FAILURE", true)
	}

	// Cache Check
	cachedAnswer, found := s.executeCacheCheckStep(ctx, inMethodLogger, &jobt, embeddingStep)
	if found {
		return returnOutput(jobt, cachedAnswer)
	}

	// Vector DB Search
	matches, err := s.executeVectorSearchStep(processContext, inMethodLogger, &jobt, embeddingStep)
	if err != nil {
		return s.jobError(jobt, err, "VECTOR_DB_FAILURE", true)
	}

	// LLM Generation
	answer, err := s.executeLLMStep(processContext, inMethodLogger, &jobt, matches, messageHistory)
	if err != nil {
		return s.jobError(jobt, err, "LLM_GENERATION_FAILURE", true)
	}

	//Background Cache Save
	go func() {
		err = s.vectorDB.SaveToCache(ctx, utils.GetNewUUID(), embeddingStep, answer)
		if err != nil {
			s.logger.Error("Failed to save to cache")
		}
	}()

	return returnOutput(jobt, answer)
}

func (s *service) IngestDocument(ctx context.Context, job jobModel.Job) jobModel.Job {
	start := time.Now()
	defer func() { metrics.CaptureExecutionMetrics("Document_ingestion", time.Since(start)) }()
	j := ingest.ProcessDocumentIngestion(ctx, job, s.embedder, s.vectorDB)
	if j.Status != jobModel.JobStatusComplete {
		return s.jobError(j, errors.New("ingest Document Failed"), "INGESTION_FAILURE", true)
	}
	return j
}
