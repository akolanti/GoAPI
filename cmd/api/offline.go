//go:build offline

package main

import (
	"context"

	"github.com/akolanti/GoAPI/internal/data/store"
	jobmodel "github.com/akolanti/GoAPI/internal/domain/jobModel"
	"github.com/akolanti/GoAPI/internal/job"
	"github.com/akolanti/GoAPI/internal/llm"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

func initOnlineServices(_ context.Context, _ llm.Provider, _ *job.Service) {}

func initStores(_ context.Context, logger *logger_i.Logger) (jobmodel.JobStore, jobmodel.MessageStore) {
	logger.Info("Offline mode: using in-memory stores")
	return store.InitInMemoryJobStore(), store.InitMessageStore()
}
