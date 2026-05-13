package ollamaEmbedding

import (
	"context"
	"fmt"
	"sync"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/rag/embedding"
	"github.com/akolanti/GoAPI/pkg/logger_i"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

var logger *logger_i.Logger
var once sync.Once
var embeddingClient *client

const batchSize = 100

type client struct {
	oai   *openai.Client
	model string
}

func GetOllamaEmbeddingClient(ctx context.Context, modelName string, apikey string) embedding.Embedder {
	once.Do(func() {
		logger = logger_i.NewLogger("ollama_embedding")
		newOllamaEmbedder(ctx, modelName, apikey)
	})

	if embeddingClient == nil {
		return nil
	}
	return &client{oai: embeddingClient.oai, model: embeddingClient.model}
}

func newOllamaEmbedder(ctx context.Context, modelName string, apikey string) {
	c := openai.NewClient(
		option.WithAPIKey(apikey),
		option.WithBaseURL(config.OllamaBaseURL),
	)

	embeddingClient = &client{oai: &c, model: modelName}
	logger.Debug("Ollama Embedding model name: " + modelName)
	logger.Info("Ollama Embedding client created")
	go closeClient(ctx, embeddingClient)
}

func closeClient(ctx context.Context, embeddingClient *client) {
	<-ctx.Done()
	logger.Info("Closing Ollama Embedding client")
	embeddingClient.oai = nil
	embeddingClient.model = ""
}

func (c *client) GetEmbedding(ctx context.Context, query string) ([]float32, error) {
	if c.oai == nil {
		return nil, fmt.Errorf("ollama embedding client is nil")
	}

	log := logger.With("traceId", ctx.Value("traceId"))
	log.Debug("query:", query)

	resp, err := c.oai.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(query),
		},
		Model: c.model,
	})
	if err != nil {
		logger.Error("Error getting embedding from Ollama:", "error", err)
		return nil, err
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned from Ollama")
	}

	return float64sToFloat32s(resp.Data[0].Embedding), nil
}

func (c *client) BatchEmbedding(ctx context.Context, chunks []string, isLargeDataSet bool) ([][]float32, error) {
	if c.oai == nil {
		return nil, fmt.Errorf("ollama embedding client is nil")
	}

	log := logger.With("traceId", ctx.Value("traceId"))

	if !isLargeDataSet {
		return c.doBatchCall(ctx, chunks)
	}

	//chunk large datasets into groups to avoid overwhelming the local server
	log.Info("Processing large dataset", "totalChunks", len(chunks))
	var allResults [][]float32

	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		batch := chunks[i:end]
		results, err := c.doBatchCall(ctx, batch)
		if err != nil {
			log.Error("Batch embedding failed", "batchStart", i, "error", err)
			return nil, err
		}
		allResults = append(allResults, results...)
	}

	return allResults, nil
}

func (c *client) doBatchCall(ctx context.Context, chunks []string) ([][]float32, error) {
	resp, err := c.oai.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: chunks,
		},
		Model: c.model,
	})
	if err != nil {
		return nil, err
	}

	results := make([][]float32, len(resp.Data))
	for _, item := range resp.Data {
		results[item.Index] = float64sToFloat32s(item.Embedding)
	}
	return results, nil
}

func float64sToFloat32s(in []float64) []float32 {
	out := make([]float32, len(in))
	for i, v := range in {
		out[i] = float32(v)
	}
	return out
}
