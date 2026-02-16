package qdrantDB

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/domain/commonModels"
	"github.com/akolanti/GoAPI/pkg/logger_i"
	"github.com/qdrant/go-client/qdrant"
)

var logger *logger_i.Logger
var quadrantInstance *qdrant.Client
var once sync.Once
var dimension = uint64(config.EmbeddingOutputDimensionality)
var collectionName = config.EmbeddingDBName

type ClientHolder struct {
	QObj *qdrant.Client
}

func GetQuadrantClient(ctx context.Context) *ClientHolder {

	once.Do(func() {
		logger = logger_i.NewLogger("Qdrant")
		res := newClient()
		if res != nil {
			quadrantInstance = res
			initCacheCollection(ctx, quadrantInstance)
			go closeQdrant(ctx, quadrantInstance)
		}
	})

	if quadrantInstance == nil {
		return nil
	}
	return &ClientHolder{
		QObj: quadrantInstance,
	}
}

func newClient() *qdrant.Client {

	host := os.Getenv("QDRANT_HOST")
	port, er := strconv.Atoi(os.Getenv("QDRANT_PORT"))

	if host == "" || er != nil {
		host = config.QdrantHost
		port = config.QdrantGrpcPort
	}

	client, err := qdrant.NewClient(&qdrant.Config{
		Host:     host,
		Port:     port,
		UseTLS:   config.QdrantUseTLS,
		PoolSize: uint(config.QdrantPoolSize),
	})
	if err != nil {
		logger.Error("could not instantiate: ", "error:", err)
	}

	err = createCollection(context.Background(), client, config.EmbeddingDBName)
	if err != nil {
		logger.Error("could not create collection: ", "collectionName", config.EmbeddingDBName, "error:", err)
		return nil
	}

	return client
}

func closeQdrant(ctx context.Context, qi *qdrant.Client) {
	<-ctx.Done()
	logger.Info("Shutting down Qdrant")
	err := qi.Close()
	if err != nil {
		logger.Error("could not close Qdrant: ", "error:", err)
	}
	logger.Info("Closed Qdrant")
}

func (db *ClientHolder) Search(ctx context.Context, vectorFloat []float32) ([]string, []string, error) {
	loggr := logger.With("traceId", ctx.Value(config.TRACE_ID_KEY))
	result, err := db.QObj.Query(ctx, &qdrant.QueryPoints{
		CollectionName: collectionName, //TODO:with access control this collection should be dynamic ie parameterized
		Query:          qdrant.NewQuery(vectorFloat...),
		Limit:          qdrant.PtrOf(uint64(3)),
		WithPayload:    qdrant.NewWithPayload(true),
	})

	if err != nil {
		loggr.Error("Error querying Qdrant: ", "error:", err)
		return nil, nil, err
	}

	//rewrite this for the structure we want for the pdf db
	var matches []string
	var metadata []string
	for _, hit := range result {
		// Check if the keys exist to avoid nil pointer panics
		content := hit.Payload["content"].GetStringValue()
		docName := hit.Payload["doc_name"].GetStringValue()
		combined := fmt.Sprintf("Content: %s, DocumentName: %s", content, docName)

		pageNum := fmt.Sprintf("page_num:%s", hit.Payload["page_num"].GetStringValue())
		docId := fmt.Sprintf("source_doc_id:%s", hit.Payload["source_doc_id"].GetStringValue())
		chunkOrder := fmt.Sprintf("chunk_order:%s", hit.Payload["chunk_order"].GetStringValue())
		chunkId := fmt.Sprintf("chunk_id:%s", hit.Payload["chunk_id"].GetStringValue())
		ingestedAt := fmt.Sprintf("ingested_at:%s", hit.Payload["ingested_at"].GetStringValue())

		metadata = append(metadata, pageNum, chunkOrder, chunkId, ingestedAt, docId)
		matches = append(matches, combined)
	}

	loggr.Debug("Found matches: %v", matches)
	return matches, metadata, nil
}

func (db *ClientHolder) CreateCollection(ctx context.Context, collectionName string) error {
	return createCollection(ctx, db.QObj, collectionName)
}

func (db *ClientHolder) UpsertBatch(ctx context.Context, collectionName string, chunks []commonModels.DocChunk, vectors [][]float32) error {
	if len(chunks) != len(vectors) {
		return fmt.Errorf("mismatch: got %d chunks but %d vectors", len(chunks), len(vectors))
	}

	qdrantPoints := make([]*qdrant.PointStruct, len(chunks))

	for i, chunk := range chunks {
		// 2. Map Chunk to Qdrant Point
		qdrantPoints[i] = &qdrant.PointStruct{
			// Converts my UUID string to Qdrant's ID format
			Id: qdrant.NewID(chunk.ChunkId),

			// Converts []float32 to Qdrant's Vector format
			Vectors: qdrant.NewVectors(vectors[i]...),

			Payload: qdrant.NewValueMap(map[string]any{
				"content":       chunk.Chunk,
				"page_num":      chunk.PageNum,
				"source_doc_id": chunk.Doc.Id,
				"doc_name":      chunk.Doc.Name,
				"chunk_order":   chunk.ChunkPageOrder,
				"chunk_id":      chunk.ChunkId,
				"ingested_at":   chunk.Doc.LastIngestTimestamp.Unix(),
			}),
		}
	}

	// 3. Perform the Upsert
	_, err := db.QObj.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collectionName,
		Points:         qdrantPoints,
		Wait:           qdrant.PtrOf(true),
	})

	if err != nil {
		return fmt.Errorf("qdrant upsert failed: %w", err)
	}

	return nil

}

func createCollection(ctx context.Context, client *qdrant.Client, collectionName string) error {
	if collectionName == "" {
		return errors.New("empty collection name")
	}

	exists, err := client.CollectionExists(ctx, collectionName)
	if err != nil {
		return err
	}
	if exists {

		return nil
	}

	err = client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: collectionName,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     dimension, //TODO:this shouldnt be hardcoded
			Distance: qdrant.Distance_Cosine,
		}),
	})
	return err
}
