package qdrantDB

import (
	"context"
	"time"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/qdrant/go-client/qdrant"
)

var semanticCacheDBName string = "semantic-cache"

func initCacheCollection(ctx context.Context, client *qdrant.Client) {
	loggr := logger.With("traceId", ctx.Value(config.TRACE_ID_KEY))
	err := createCollection(ctx, client, semanticCacheDBName)
	if err != nil {
		loggr.Error("Semantic cache collection creation failed", "error", err)
	}
}

func (db *ClientHolder) GetCachedAnswer(ctx context.Context, queryVector []float32) (string, bool, error) {
	loggr := logger.With("traceId", ctx.Value(config.TRACE_ID_KEY))

	loggr.Info("Searching for cached answer")
	searchResult, err := db.QObj.Query(ctx, &qdrant.QueryPoints{
		CollectionName: semanticCacheDBName,
		Query:          qdrant.NewQuery(queryVector...),
		Limit:          qdrant.PtrOf(uint64(1)),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil || len(searchResult) == 0 {
		loggr.Error("Cache Query failed", "error", err)
		return "", false, err
	}

	loggr.Debug("Found cached answer", "semantic similarity score", searchResult[0].Score)
	// Threshold Check: 0.95 is a safe "semantic match"
	if searchResult[0].Score < config.CacheSimilarityCutoff {
		return "", false, nil
	}

	loggr.Info("---------------cache hit---------------------")
	// Extract the answer from your JobPayload structure stored in payload
	answer := searchResult[0].Payload["answer"].GetStringValue()
	return answer, true, nil
}

func (db *ClientHolder) SaveToCache(ctx context.Context, id string, vector []float32, answer string) error {
	loggr := logger.With("traceId", ctx.Value(config.TRACE_ID_KEY))

	loggr.Debug("Saving answer to cache")
	_, err := db.QObj.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: "semantic-cache",
		Points: []*qdrant.PointStruct{
			{
				Id:      qdrant.NewID(id),
				Vectors: qdrant.NewVectors(vector...),
				Payload: qdrant.NewValueMap(map[string]any{
					"answer":    answer,
					"timestamp": time.Now().Unix(),
				}),
			},
		},
	})
	if err != nil {
		loggr.Error("Saving answer to cache failed", "error", err)
	}
	return err
}
