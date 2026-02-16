package rag_test

import (
	"context"

	"github.com/akolanti/GoAPI/internal/domain/commonModels"
)

// MockVectorDB implements vectorDB.DataProcessor
type MockVectorDB struct {
	// Control fields to simulate different behaviors
	OnSearch           func(ctx context.Context, vectorVal []float32) ([]string, error)
	OnGetCachedAnswer  func(ctx context.Context, queryVector []float32) (string, bool, error)
	OnSaveToCache      func(ctx context.Context, id string, vector []float32, answer string) error
	OnCreateCollection func(ctx context.Context, name string) error
	OnUpsertBatch      func(ctx context.Context, name string, chunks []commonModels.DocChunk, vectors [][]float32) error
}

func (m *MockVectorDB) Search(ctx context.Context, v []float32) ([]string, error) {
	if m.OnSearch != nil {
		return m.OnSearch(ctx, v)
	}
	return []string{"default context"}, nil
}

func (m *MockVectorDB) GetCachedAnswer(ctx context.Context, v []float32) (string, bool, error) {
	if m.OnGetCachedAnswer != nil {
		return m.OnGetCachedAnswer(ctx, v)
	}
	return "", false, nil
}

func (m *MockVectorDB) SaveToCache(ctx context.Context, id string, v []float32, a string) error {
	if m.OnSaveToCache != nil {
		return m.OnSaveToCache(ctx, id, v, a)
	}
	return nil
}

func (m *MockVectorDB) CreateCollection(ctx context.Context, name string) error {
	if m.OnCreateCollection != nil {
		return m.OnCreateCollection(ctx, name)
	}
	return nil
}

func (m *MockVectorDB) UpsertBatch(ctx context.Context, name string, chunks []commonModels.DocChunk, vectors [][]float32) error {
	if m.OnUpsertBatch != nil {
		return m.OnUpsertBatch(ctx, name, chunks, vectors)
	}
	return nil
}

type MockEmbedder struct {
	OnGetEmbedding   func(ctx context.Context, text string) ([]float32, error)
	OnBatchEmbedding func(ctx context.Context, chunks []string, isHuge bool) ([][]float32, error)
}

func (m *MockEmbedder) BatchEmbedding(ctx context.Context, chunks []string, isHuge bool) ([][]float32, error) {
	if m.OnBatchEmbedding != nil {
		return m.OnBatchEmbedding(ctx, chunks, isHuge)
	}
	// Return dummy vectors matching chunk size
	return make([][]float32, len(chunks)), nil
}

func (m *MockEmbedder) GetEmbedding(ctx context.Context, query string) ([]float32, error) {
	return []float32{0.1}, nil
}

// MockLLM implements llm.Provider
type MockLLM struct {
	OnGenerate func(ctx context.Context, query string, matches []string, history []string) (string, error)
}

func (m *MockLLM) Generate(ctx context.Context, q string, mth []string, hist []string) (string, error) {
	if m.OnGenerate != nil {
		return m.OnGenerate(ctx, q, mth, hist)
	}
	return "mocked llm response", nil
}
