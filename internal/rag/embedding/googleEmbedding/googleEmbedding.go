package googleEmbedding

import (
	"context"
	"sync"
	"time"

	"github.com/akolanti/GoAPI/internal/adapter/utils"
	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/rag/embedding"
	"github.com/akolanti/GoAPI/pkg/logger_i"
	"google.golang.org/genai"
)

var logger *logger_i.Logger
var once sync.Once
var embeddingClient *client
var dimension int32 = config.EmbeddingOutputDimensionality

type client struct {
	genAi *genai.Client
	model string
}

func newGoogleEmbedder(ctx context.Context, modelName string, apikey string) {
	c, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apikey})
	if err != nil {
		logger.Error("Error creating Google Embedding client:", err)
	}
	if c != nil {
		embeddingClient = &client{
			genAi: c,
			model: modelName,
		}
		logger.Debug("Google Embedding model name: " + modelName)
		logger.Info("Google Embedding client created")
		go closeClient(ctx, embeddingClient)
	}
}

func closeClient(ctx context.Context, embeddingClient *client) {
	<-ctx.Done()
	logger.Info("Closing Google Embedding client")
	embeddingClient.genAi = nil
	embeddingClient.model = ""
}

func GetGoogleEmbeddingClient(ctx context.Context, modelName string, apikey string) embedding.Embedder {
	once.Do(func() {
		logger = logger_i.NewLogger("google_embedding")
		newGoogleEmbedder(ctx, modelName, apikey)
	})

	//if init still fails
	if embeddingClient == nil {
		return nil
	}
	return &client{genAi: embeddingClient.genAi, model: embeddingClient.model}
}

func (c *client) GetEmbedding(ctx context.Context, query string) ([]float32, error) {
	log := logger.With("traceId", ctx.Value("traceId"))
	log.Debug("query:", query)
	//result, err := c.doCall(ctx, genai.Text(query))
	text := genai.Text(query)

	result, err := c.genAi.Models.EmbedContent(ctx, c.model, text, &genai.EmbedContentConfig{OutputDimensionality: &dimension, TaskType: "RETRIEVAL_DOCUMENT"})
	if err != nil {
		log.Error("Error getting regular Embeddings from Google", err.Error())
		return nil, err
	}
	return result.Embeddings[0].Values, nil
}

func (c *client) BatchEmbedding(ctx context.Context, chunks []string, isLargeDataSet bool) ([][]float32, error) {
	log := logger_i.NewLogger("batch_embedding")
	log = logger.With("trace Id", ctx.Value("traceId").(string))

	if !isLargeDataSet {
		res, err := c.doCall(ctx, getContent(chunks))
		if err != nil || res == nil {
			if doRetry(err, logger) {
				time.Sleep(5 * time.Second)
				log.Debug("Retrying in 5 seconds")

				res, err = c.doCall(ctx, getContent(chunks))
				if err != nil {
					logger.Error("Error getting Embedding from Google", "error", err.Error())
				}
			}
			log.Error("Error getting Embeddings from Google", "error", err, "res", res)
			return nil, err
		}
		var embeddingResults [][]float32
		for _, r := range res.Embeddings {
			embeddingResults = append(embeddingResults, r.Values)
		}

		return embeddingResults, nil
	}

	t1 := genai.EmbeddingsBatchJobSource{InlinedRequests: getInlinedBatchRequests(chunks)}
	batchJobName := utils.GetNewUUID()

	log = logger.With("batchJobName", batchJobName, "big file", len(chunks))
	conf := genai.CreateEmbeddingsBatchJobConfig{DisplayName: batchJobName}
	_, err := c.genAi.Batches.CreateEmbeddings(ctx, &c.model, &t1, &conf)
	if err != nil {
		log.Error("Error getting batch Embeddings from Google", "error", err.Error())
		return nil, err
	}

	answer, err := c.pollForAnswer(ctx, batchJobName, log)
	if err != nil {
		return nil, err
	}
	resultVectors, downErrors := downloadAnswerFromClient(answer, log)

	if downErrors != nil {
		log.Error("Error downloading answers from Google Embedding client: ", "errors", downErrors)
	}

	return resultVectors, nil

}

func (c *client) doCall(ctx context.Context, content []*genai.Content) (*genai.EmbedContentResponse, error) {
	result, err := c.genAi.Models.EmbedContent(ctx, c.model, content, &genai.EmbedContentConfig{OutputDimensionality: &dimension, TaskType: "RETRIEVAL_DOCUMENT"})
	return result, err
}
