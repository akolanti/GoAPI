//go:build !offline

package main

import (
	"context"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/data/store"
	jobmodel "github.com/akolanti/GoAPI/internal/domain/jobModel"
	"github.com/akolanti/GoAPI/internal/job"
	"github.com/akolanti/GoAPI/internal/llm"
	"github.com/akolanti/GoAPI/internal/mcpImpl"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

func initOnlineServices(ctx context.Context, llmProvider llm.Provider, service *job.Service) {
	if !config.OFFLINE_MODE {
		mcpImpl.InitMCPHandler(ctx, llmProvider, service)
	}
}

func initStores(ctx context.Context, logger *logger_i.Logger) (jobmodel.JobStore, jobmodel.MessageStore) {
	if config.OFFLINE_MODE {
		logger.Info("Local mode: using in-memory stores")
		return store.InitInMemoryJobStore(), store.InitMessageStore()
	}
	jobStore := store.GetRedisJobStore(ctx)
	msgStore := store.GetRedisMessageStore(ctx)
	if jobStore == nil || msgStore == nil {
		logger.Error("Redis stores are offline, falling back to in-memory stores")
		return store.InitInMemoryJobStore(), store.InitMessageStore()
	}
	return jobStore, msgStore
}
