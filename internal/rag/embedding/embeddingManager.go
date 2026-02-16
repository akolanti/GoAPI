package embedding

import "context"

type Embedder interface {
	GetEmbedding(ctx context.Context, query string) ([]float32, error)
	BatchEmbedding(ctx context.Context, chunks []string, isHugeDataSet bool) ([][]float32, error)
}
