package store

import (
	"context"
	"sync"

	"github.com/akolanti/GoAPI/internal/domain/jobModel"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

var inMemLogger = logger_i.NewLogger("InMem JobStore")

type InMemoryJobStore struct {
	jobMutex *sync.RWMutex
	jobMap   map[string]jobModel.Job
}

func InitInMemoryJobStore() *InMemoryJobStore {
	return &InMemoryJobStore{
		jobMutex: new(sync.RWMutex),
		jobMap:   make(map[string]jobModel.Job),
	}
}

func (store *InMemoryJobStore) SaveJob(ctx context.Context, jobToStored jobModel.Job) error {

	store.jobMutex.Lock()
	defer store.jobMutex.Unlock()
	store.jobMap[jobToStored.Id] = jobToStored
	inMemLogger.Info(jobToStored.Id, " : Saved job to store")
	return nil
}

func (store *InMemoryJobStore) GetJob(ctx context.Context, jobId string) (jobModel.Job, bool) {
	store.jobMutex.RLock()
	defer store.jobMutex.RUnlock()
	result, found := store.jobMap[jobId]
	inMemLogger.Info(jobId, " : Is job found :", found)
	return result, found
}

func (store *InMemoryJobStore) DeleteJob(ctx context.Context, jobID string) {
	store.jobMutex.Lock()
	defer store.jobMutex.Unlock()
	delete(store.jobMap, jobID)
}
