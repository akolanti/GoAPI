//go:build offline

package factory

import (
	"context"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/rag/embedding"
	"github.com/akolanti/GoAPI/internal/rag/embedding/ollamaEmbedding"
)

func NewEmbedder(ctx context.Context) embedding.Embedder {
	return ollamaEmbedding.GetOllamaEmbeddingClient(ctx, config.OllamaEmbeddingModel, config.OllamaAPIKey)
}
