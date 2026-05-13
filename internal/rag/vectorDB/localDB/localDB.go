package localDB

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/domain/commonModels"
	"github.com/akolanti/GoAPI/internal/rag/vectorDB"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

var logger *logger_i.Logger
var localInstance *store
var once sync.Once

var collectionName = config.EmbeddingDBName
var semanticCacheDBName = "semantic-cache"

type point struct {
	Vector  []float32      `json:"vector"`
	Payload map[string]any `json:"payload"`
}

type collection struct {
	Points map[string]point `json:"points"`
}

type store struct {
	Collections map[string]*collection `json:"collections"`
	mu          sync.RWMutex
	dbPath      string
	cachePath   string
}

func GetLocalClient(ctx context.Context) vectorDB.DataProcessor {
	once.Do(func() {
		logger = logger_i.NewLogger("local_vectordb")
		localInstance = newStore(ctx)
	})

	if localInstance == nil {
		return nil
	}
	return localInstance
}

func newStore(ctx context.Context) *store {
	s := &store{
		Collections: make(map[string]*collection),
		dbPath:      config.LocalVectorDBPath,
		cachePath:   config.LocalCacheDBPath,
	}

	//load existing data from files
	loadFromFile(s.dbPath, s)
	loadCacheFromFile(s.cachePath, s)

	//ensure main collection exists
	if s.Collections[collectionName] == nil {
		s.Collections[collectionName] = &collection{Points: make(map[string]point)}
	}
	//ensure cache collection exists
	if s.Collections[semanticCacheDBName] == nil {
		s.Collections[semanticCacheDBName] = &collection{Points: make(map[string]point)}
	}

	logger.Info("Local vector DB initialized", "dbPath", s.dbPath, "cachePath", s.cachePath)
	go closeClient(ctx, s)
	return s
}

func closeClient(ctx context.Context, s *store) {
	<-ctx.Done()
	logger.Info("Shutting down local vector DB, persisting data")
	s.persist()
	logger.Info("Local vector DB shut down")
}

func (s *store) Search(ctx context.Context, vectorFloat []float32) ([]string, []string, error) {
	loggr := logger.With("traceId", ctx.Value(config.TRACE_ID_KEY))

	s.mu.RLock()
	col, exists := s.Collections[collectionName]
	s.mu.RUnlock()

	if !exists || len(col.Points) == 0 {
		loggr.Debug("No points in collection", "collection", collectionName)
		return nil, nil, nil
	}

	type scored struct {
		id    string
		score float32
		p     point
	}

	var results []scored
	for id, p := range col.Points {
		score := cosineSimilarity(vectorFloat, p.Vector)
		results = append(results, scored{id: id, score: score, p: p})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	//I need to figure out a way to do ranking

	limit := 1
	if len(results) < limit {
		limit = len(results)
	}

	var matches []string
	var metadata []string
	for _, hit := range results[:limit] {
		content := payloadString(hit.p.Payload, "content")
		docName := payloadString(hit.p.Payload, "doc_name")
		combined := fmt.Sprintf("Content: %s, DocumentName: %s", content, docName)

		docNameMeta := fmt.Sprintf("doc_name:%s", payloadString(hit.p.Payload, "doc_name"))
		pageNum := fmt.Sprintf("page_num:%s", payloadString(hit.p.Payload, "page_num"))
		docId := fmt.Sprintf("source_doc_id:%s", payloadString(hit.p.Payload, "source_doc_id"))
		chunkOrder := fmt.Sprintf("chunk_order:%s", payloadString(hit.p.Payload, "chunk_order"))
		chunkId := fmt.Sprintf("chunk_id:%s", payloadString(hit.p.Payload, "chunk_id"))
		ingestedAt := fmt.Sprintf("ingested_at:%s", payloadString(hit.p.Payload, "ingested_at"))
		score := fmt.Sprintf("score:%f", hit.score)

		metadata = append(metadata, docNameMeta, pageNum, chunkOrder, chunkId, ingestedAt, docId, score)
		matches = append(matches, combined)
	}

	loggr.Debug("Found matches", "count", len(matches))
	return matches, metadata, nil
}

func (s *store) GetCachedAnswer(ctx context.Context, queryVector []float32) (string, bool, error) {
	loggr := logger.With("traceId", ctx.Value(config.TRACE_ID_KEY))

	loggr.Info("Searching for cached answer")

	s.mu.RLock()
	col, exists := s.Collections[semanticCacheDBName]
	s.mu.RUnlock()

	if !exists || len(col.Points) == 0 {
		return "", false, nil
	}

	var bestScore float32
	var bestAnswer string
	for _, p := range col.Points {
		score := cosineSimilarity(queryVector, p.Vector)
		if score > bestScore {
			bestScore = score
			bestAnswer = payloadString(p.Payload, "answer")
		}
	}

	loggr.Debug("Found cached answer", "semantic similarity score", bestScore)
	if bestScore < config.CacheSimilarityCutoff {
		return "", false, nil
	}

	loggr.Info("---------------cache hit---------------------")
	return bestAnswer, true, nil
}

func (s *store) SaveToCache(ctx context.Context, id string, vector []float32, answer string) error {
	loggr := logger.With("traceId", ctx.Value(config.TRACE_ID_KEY))

	loggr.Debug("Saving answer to cache")

	s.mu.Lock()
	if s.Collections[semanticCacheDBName] == nil {
		s.Collections[semanticCacheDBName] = &collection{Points: make(map[string]point)}
	}
	s.Collections[semanticCacheDBName].Points[id] = point{
		Vector: vector,
		Payload: map[string]any{
			"answer":    answer,
			"timestamp": time.Now().Unix(),
		},
	}
	s.mu.Unlock()

	s.persistCache()
	return nil
}

func (s *store) CreateCollection(ctx context.Context, collectionName string) error {
	if collectionName == "" {
		return errors.New("empty collection name")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Collections[collectionName] != nil {
		return nil
	}

	s.Collections[collectionName] = &collection{Points: make(map[string]point)}
	logger.Info("Created collection", "name", collectionName)
	return nil
}

func (s *store) UpsertBatch(ctx context.Context, collectionName string, chunks []commonModels.DocChunk, vectors [][]float32) error {
	if len(chunks) != len(vectors) {
		return fmt.Errorf("mismatch: got %d chunks but %d vectors", len(chunks), len(vectors))
	}

	s.mu.Lock()
	if s.Collections[collectionName] == nil {
		s.Collections[collectionName] = &collection{Points: make(map[string]point)}
	}
	col := s.Collections[collectionName]

	for i, chunk := range chunks {
		col.Points[chunk.ChunkId] = point{
			Vector: vectors[i],
			Payload: map[string]any{
				"content":       chunk.Chunk,
				"page_num":      chunk.PageNum,
				"source_doc_id": chunk.Doc.Id,
				"doc_name":      chunk.Doc.Name,
				"chunk_order":   chunk.ChunkPageOrder,
				"chunk_id":      chunk.ChunkId,
				"ingested_at":   chunk.Doc.LastIngestTimestamp.Unix(),
			},
		}
	}
	s.mu.Unlock()

	s.persist()
	return nil
}
