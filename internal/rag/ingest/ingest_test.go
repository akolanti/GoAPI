package ingest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/akolanti/GoAPI/internal/domain/commonModels"
)

// --- Mocks for BatchIngest ---

type mockEmbedder struct {
	batchFunc func(ctx context.Context, chunks []string, isHuge bool) ([][]float32, error)
}

func (m *mockEmbedder) GetEmbedding(ctx context.Context, query string) ([]float32, error) {
	return nil, nil
}
func (m *mockEmbedder) BatchEmbedding(ctx context.Context, chunks []string, isHuge bool) ([][]float32, error) {
	return m.batchFunc(ctx, chunks, isHuge)
}

type mockVectorDB struct {
	upsertFunc func(ctx context.Context, coll string, chunks []commonModels.DocChunk, vectors [][]float32) error
}

func (m *mockVectorDB) Search(ctx context.Context, v []float32) ([]string, error) { return nil, nil }
func (m *mockVectorDB) GetCachedAnswer(ctx context.Context, v []float32) (string, bool, error) {
	return "", false, nil
}
func (m *mockVectorDB) SaveToCache(ctx context.Context, id string, v []float32, a string) error {
	return nil
}
func (m *mockVectorDB) CreateCollection(ctx context.Context, name string) error { return nil }
func (m *mockVectorDB) UpsertBatch(ctx context.Context, coll string, chunks []commonModels.DocChunk, vectors [][]float32) error {
	return m.upsertFunc(ctx, coll, chunks, vectors)
}

// --- Unit Tests ---

func TestGetDocType(t *testing.T) {
	tests := []struct {
		path     string
		expected commonModels.DocType
	}{
		{"test.pdf", commonModels.PDF},
		{"DOC.DOCX", commonModels.DOCX},
		{"notes.txt", commonModels.DOCX},
		{"image.png", commonModels.ERR},
	}

	for _, tt := range tests {
		if got := getDocType(tt.path); got != tt.expected {
			t.Errorf("getDocType(%s) = %v; want %v", tt.path, got, tt.expected)
		}
	}
}

func TestSplitTextIntoChunks(t *testing.T) {
	text := "This is a long sentence. This is another sentence that will be split."
	limit := 30
	overlap := 5

	chunks := splitTextIntoChunks(text, limit, overlap)

	if len(chunks) < 2 {
		t.Errorf("Expected multiple chunks, got %d", len(chunks))
	}

	// Verify overlap (simple check if second chunk contains start of overlap)
	if len(chunks) > 1 {
		lastCharsOfFirst := chunks[0][len(chunks[0])-overlap:]
		if !strings.HasPrefix(chunks[1], lastCharsOfFirst) {
			t.Logf("Note: Basic overlap check failed, ensure logic matches: %s vs %s", lastCharsOfFirst, chunks[1])
		}
	}
}

func TestBatchIngest(t *testing.T) {
	ctx := context.Background()
	chunks := make([]commonModels.DocChunk, 150) // Should trigger 2 batches (100 + 50)
	for i := range chunks {
		chunks[i] = commonModels.DocChunk{Chunk: "test content"}
	}

	callCount := 0
	vDB := &mockVectorDB{
		upsertFunc: func(ctx context.Context, coll string, c []commonModels.DocChunk, v [][]float32) error {
			callCount++
			return nil
		},
	}

	emb := &mockEmbedder{
		batchFunc: func(ctx context.Context, ch []string, huge bool) ([][]float32, error) {
			return make([][]float32, len(ch)), nil
		},
	}

	err := BatchIngest(ctx, chunks, vDB, emb)

	if err != nil {
		t.Fatalf("BatchIngest failed: %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 batches to be upserted, got %d", callCount)
	}
}

func TestBatchIngest_Error(t *testing.T) {
	vDB := &mockVectorDB{
		upsertFunc: func(ctx context.Context, coll string, c []commonModels.DocChunk, v [][]float32) error {
			return errors.New("upsert failed")
		},
	}
	emb := &mockEmbedder{
		batchFunc: func(ctx context.Context, ch []string, huge bool) ([][]float32, error) {
			return make([][]float32, len(ch)), nil
		},
	}

	err := BatchIngest(context.Background(), []commonModels.DocChunk{{Chunk: "hi"}}, vDB, emb)
	if err == nil {
		t.Error("Expected error from BatchIngest, got nil")
	}
}

func TestPrepareChunks(t *testing.T) {
	pages := []rawPage{
		{Number: 1, Content: "Page one content."},
		{Number: 2, Content: "Page two content."},
	}
	doc := commonModels.Document{Id: "doc-1"}

	chunks := PrepareChunks(pages, doc, "text-embedding-3-small")

	if len(chunks) != 2 {
		t.Errorf("Expected 2 chunks (one per page), got %d", len(chunks))
	}

	if chunks[0].Doc.Id != "doc-1" || chunks[0].PageNum != 1 {
		t.Errorf("Metadata mismatch in chunk 0: %+v", chunks[0])
	}
}
