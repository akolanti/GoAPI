package store_test

import (
	"context"
	"testing"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/data/redisStore"
	"github.com/akolanti/GoAPI/internal/data/store" //
	"github.com/akolanti/GoAPI/internal/domain/jobModel"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisJobStore_Lifecycle(t *testing.T) {
	// 1. Start miniredis
	mr := miniredis.RunT(t)

	//I simply dont want to expose stuff to other classes about the store being used
	//this is a sacrifice that I will make temporarily
	
	//	internalStore := redisStore.NewTestStore(client)
	//	jobStore := store.TestJobStore(internalStore)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	internalStore := redisStore.NewTestStore(client)
	jobStore := store.TestJobStore(internalStore)

	ctx := context.WithValue(context.Background(), config.TRACE_ID_KEY, "test-trace")
	jobID := "job_abc_123"

	testJob := jobModel.Job{
		Id:     jobID,
		Status: jobModel.JobStatusRunning,
		JobPayload: jobModel.JobPayload{
			Question: "How do I mock Redis?",
		},
	}

	t.Run("Save and Get Roundtrip", func(t *testing.T) {
		// Test Save
		err := jobStore.SaveJob(ctx, testJob)
		if err != nil {
			t.Fatalf("SaveJob failed: %v", err)
		}

		// Test Get
		retrievedJob, found := jobStore.GetJob(ctx, jobID)
		if !found {
			t.Fatal("Job was saved but not found in Redis")
		}

		if retrievedJob.JobPayload.Question != testJob.JobPayload.Question {
			t.Errorf("Data mismatch! Got %s, want %s",
				retrievedJob.JobPayload.Question, testJob.JobPayload.Question)
		}
	})

	t.Run("Get Non-Existent Job", func(t *testing.T) {
		_, found := jobStore.GetJob(ctx, "ghost-id")
		if found {
			t.Error("Expected found=false for non-existent key")
		}
	})

	t.Run("Delete Job", func(t *testing.T) {
		jobStore.DeleteJob(ctx, jobID)

		// Verify it's gone from miniredis
		if mr.Exists(jobID) {
			t.Error("Job still exists in Redis after DeleteJob call")
		}
	})
}

func TestRedisJobStore_Race(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	jobStore := store.TestJobStore(redisStore.NewTestStore(client))

	ctx := context.WithValue(context.Background(), config.TRACE_ID_KEY, "race-trace")
	job := jobModel.Job{Id: "race-job"}

	const workers = 50
	for i := 0; i < workers; i++ {
		go func() {
			_ = jobStore.SaveJob(ctx, job)
			_, _ = jobStore.GetJob(ctx, "race-job")
		}()
	}
}
