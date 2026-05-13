//go:build !offline

package factory

import (
	"context"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/rag/vectorDB"
	"github.com/akolanti/GoAPI/internal/rag/vectorDB/localDB"
	"github.com/akolanti/GoAPI/internal/rag/vectorDB/qdrantDB"
)

func NewVectorDB(ctx context.Context) vectorDB.DataProcessor {
	if config.OFFLINE_MODE {
		return localDB.GetLocalClient(ctx)
	}
	return qdrantDB.GetQuadrantClient(ctx)
}
