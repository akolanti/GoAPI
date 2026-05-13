//go:build offline

package factory

import (
	"context"

	"github.com/akolanti/GoAPI/internal/rag/vectorDB"
	"github.com/akolanti/GoAPI/internal/rag/vectorDB/localDB"
)

func NewVectorDB(ctx context.Context) vectorDB.DataProcessor {
	return localDB.GetLocalClient(ctx)
}
