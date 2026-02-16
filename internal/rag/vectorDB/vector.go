package vectorDB

import (
	"context"

	"github.com/akolanti/GoAPI/internal/domain/commonModels"
)

type DataProcessor interface {
	Search(ctx context.Context, vectorVal []float32) ([]string, []string, error)
	GetCachedAnswer(ctx context.Context, queryVector []float32) (string, bool, error)
	SaveToCache(ctx context.Context, id string, vector []float32, answer string) error

	// CreateCollection Ingest document call
	CreateCollection(ctx context.Context, collectionName string) error
	UpsertBatch(ctx context.Context, collectionName string, chunks []commonModels.DocChunk, vectors [][]float32) error
}
