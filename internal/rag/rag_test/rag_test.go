package rag_test

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/domain/commonModels"
	"github.com/akolanti/GoAPI/internal/domain/jobModel"
	"github.com/akolanti/GoAPI/internal/rag"
)

func TestProcessRequest_Scenarios(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(e *MockEmbedder, v *MockVectorDB, l *MockLLM)
		expectedStep   jobModel.InternalStatus
		expectedStatus jobModel.JobStatus
		expectedAnswer string
		expectedErr    string
	}{
		{
			name: "Success_Full_Flow",
			setupMocks: func(e *MockEmbedder, v *MockVectorDB, l *MockLLM) {
				v.OnGetCachedAnswer = func(ctx context.Context, emb []float32) (string, bool, error) {
					return "", false, nil
				}
				l.OnGenerate = func(ctx context.Context, q string, m []string, h []string) (string, error) {
					return "final answer", nil
				}
			},
			expectedStep:   jobModel.Complete,
			expectedStatus: jobModel.JobStatusQueued,
			expectedAnswer: "final answer",
		},
		{
			name: "Success_Cache_Hit",
			setupMocks: func(e *MockEmbedder, v *MockVectorDB, l *MockLLM) {
				v.OnGetCachedAnswer = func(ctx context.Context, emb []float32) (string, bool, error) {
					return "cached answer", true, nil
				}
			},
			expectedStep:   jobModel.Complete,
			expectedStatus: jobModel.JobStatusQueued,
			expectedAnswer: "cached answer",
		},
		{
			name: "Failure_Embedding",
			setupMocks: func(e *MockEmbedder, v *MockVectorDB, l *MockLLM) {
				e.OnGetEmbedding = func(ctx context.Context, text string) ([]float32, error) {
					return nil, errors.New("api limit")
				}
			},
			expectedStatus: jobModel.JobStatusError,
			expectedErr:    "EMBEDDING_FAILURE",
		},
		{
			name: "Failure_Vector_Search",
			setupMocks: func(e *MockEmbedder, v *MockVectorDB, l *MockLLM) {
				v.OnGetCachedAnswer = func(ctx context.Context, emb []float32) (string, bool, error) {
					return "", false, nil
				}
				v.OnSearch = func(ctx context.Context, v []float32) ([]string, error) {
					return nil, errors.New("db timeout")
				}
			},
			expectedStatus: jobModel.JobStatusError,
			expectedErr:    "VECTOR_DB_FAILURE",
		},
		{
			name: "Failure_LLM_Generation",
			setupMocks: func(e *MockEmbedder, v *MockVectorDB, l *MockLLM) {
				v.OnGetCachedAnswer = func(ctx context.Context, emb []float32) (string, bool, error) {
					return "", false, nil
				}
				l.OnGenerate = func(ctx context.Context, q string, m []string, h []string) (string, error) {
					return "", errors.New("provider down")
				}
			},
			expectedStatus: jobModel.JobStatusError,
			expectedErr:    "LLM_GENERATION_FAILURE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mEmbed := &MockEmbedder{}
			mVec := &MockVectorDB{}
			mLLM := &MockLLM{}

			tt.setupMocks(mEmbed, mVec, mLLM)

			s := rag.NewService(mVec, mLLM, mEmbed)

			ctx := context.WithValue(context.Background(), config.TRACE_ID_KEY, "test-trace")
			job := jobModel.Job{
				Id: "test-job",
				JobPayload: jobModel.JobPayload{
					Question: "test question",
				},
			}

			result := s.ProcessRequest(ctx, job, []string{})

			if result.Status != tt.expectedStatus {
				t.Errorf("Step got %v, want %v", result.Status, tt.expectedStatus)
			}

			if tt.expectedAnswer != "" && result.JobPayload.Answer != tt.expectedAnswer {
				t.Errorf("Answer got %s, want %s", result.JobPayload.Answer, tt.expectedAnswer)
			}

			if tt.expectedErr != "" && result.Error.Code != http.StatusInternalServerError {
				t.Errorf("Error Code got %d, want %s", result.Error.Code, tt.expectedErr)
			}
		})
	}
}

func TestIngestDocument_Scenarios(t *testing.T) {
	dummyFile := "test_ingest.txt"
	os.WriteFile(dummyFile, []byte("test content for ingestion"), 0644)
	defer os.Remove(dummyFile)

	tests := []struct {
		name           string
		setupMocks     func(e *MockEmbedder, v *MockVectorDB)
		expectedStatus jobModel.JobStatus
		expectedErr    string
	}{
		{
			name: "Ingestion_Success",
			setupMocks: func(e *MockEmbedder, v *MockVectorDB) {
				v.OnCreateCollection = func(ctx context.Context, name string) error {
					return nil
				}
				v.OnUpsertBatch = func(ctx context.Context, coll string, chunks []commonModels.DocChunk, vectors [][]float32) error {
					return nil
				}
				e.OnBatchEmbedding = func(ctx context.Context, chunks []string, isHuge bool) ([][]float32, error) {
					return make([][]float32, len(chunks)), nil
				}
			},
			expectedStatus: jobModel.JobStatusComplete,
		},
		{
			name: "Failure_Collection_Creation",
			setupMocks: func(e *MockEmbedder, v *MockVectorDB) {
				v.OnCreateCollection = func(ctx context.Context, name string) error {
					return errors.New("connection refused")
				}
			},
			expectedStatus: jobModel.JobStatusError,
			expectedErr:    "INGESTION_FAILURE",
		},
		{
			name: "Failure_Batch_Upsert",
			setupMocks: func(e *MockEmbedder, v *MockVectorDB) {
				v.OnCreateCollection = func(ctx context.Context, name string) error {
					return nil
				}
				e.OnBatchEmbedding = func(ctx context.Context, chunks []string, isHuge bool) ([][]float32, error) {
					return make([][]float32, len(chunks)), nil
				}
				v.OnUpsertBatch = func(ctx context.Context, coll string, chunks []commonModels.DocChunk, vectors [][]float32) error {
					return errors.New("disk full")
				}
			},
			expectedStatus: jobModel.JobStatusError,
			expectedErr:    "INGESTION_FAILURE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mEmbed := &MockEmbedder{}
			mVec := &MockVectorDB{}

			tt.setupMocks(mEmbed, mVec)

			s := rag.NewService(mVec, &MockLLM{}, mEmbed)

			ctx := context.WithValue(context.Background(), config.TRACE_ID_KEY, "ingest-trace")
			job := jobModel.Job{
				Id: "ingest-job-1",
				JobPayload: jobModel.JobPayload{
					IngestFileName: "test_ingest.txt",
					IngestURL:      dummyFile,
				},
			}

			// Re-create file if deleted by previous successful test run
			if _, err := os.Stat(dummyFile); os.IsNotExist(err) {
				os.WriteFile(dummyFile, []byte("test content"), 0644)
			}

			result := s.IngestDocument(ctx, job)

			if result.Status != tt.expectedStatus {
				t.Errorf("Status got %v, want %v", result.Status, tt.expectedStatus)
			}

			if tt.expectedErr != "" && result.Error.Code != http.StatusInternalServerError {
				t.Errorf("Error Code got %d, want %s", result.Error.Code, tt.expectedErr)
			}
		})
	}
}
