package store

import (
	"context"
	"encoding/json"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/data/redisStore"
	"github.com/akolanti/GoAPI/internal/domain/jobModel"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

type RedisJobStore struct {
	store  *redisStore.Store
	logger *logger_i.Logger
}

func GetRedisJobStore(ctx context.Context) *RedisJobStore {
	return &RedisJobStore{
		store:  redisStore.GetRedisStore(ctx, config.RedisJobStore),
		logger: logger_i.NewLogger("JobStore"),
	}
}

func (s *RedisJobStore) SaveJob(ctx context.Context, job jobModel.Job) error {
	log := s.logger.With("traceId", ctx.Value(config.TRACE_ID_KEY), "job Id", job.Id)
	log.Debug("saving job")
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}

	err = s.store.Set(ctx, job.Id, data, config.RedisJobStoreTTL)
	if err == nil {
		log.Debug("Saved job to Redis")
	}
	return err
}

func (s *RedisJobStore) GetJob(ctx context.Context, jobId string) (jobModel.Job, bool) {
	var job jobModel.Job
	log := s.logger.With("traceId", ctx.Value(config.TRACE_ID_KEY), "job Id", jobId)
	log.Debug("getting job")
	val, err := s.store.Get(ctx, jobId)
	if s.store.IsNil(err) {
		return job, false
	} else if err != nil {
		return job, false
	}

	log.Debug("Unmarshalling job")
	// 2. Unmarshal JSON back into the Job struct
	err = json.Unmarshal([]byte(val), &job)
	if err != nil {
		return job, false
	}

	log.Debug(": Job found in Redis")
	return job, true
}

func (s *RedisJobStore) DeleteJob(ctx context.Context, jobID string) {
	err := s.store.Del(ctx, jobID)
	if err != nil {
		s.logger.Error(jobID, "jobId", ": Error deleting job from Redis")
		return
	}
	s.logger.Debug(" Job deleted from Redis", "jobId:", jobID)
}

func TestJobStore(store *redisStore.Store) *RedisJobStore {
	return &RedisJobStore{
		store:  store,
		logger: logger_i.NewLogger("test redis"),
	}
}
