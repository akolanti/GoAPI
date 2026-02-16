package worker

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/domain/jobModel"
	"github.com/akolanti/GoAPI/internal/job"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

// MockRagService to track if jobs are executed
type MockRagService struct {
	ProcessedCount int32
}

func (m *MockRagService) ProcessRequest(ctx context.Context, j jobModel.Job, hist []string) jobModel.Job {
	atomic.AddInt32(&m.ProcessedCount, 1)
	return j
}

func (m *MockRagService) IngestDocument(ctx context.Context, j jobModel.Job) jobModel.Job {
	atomic.AddInt32(&m.ProcessedCount, 1)
	return j
}

type MockJobStore struct {
	OnSaveJob func(ctx context.Context, job jobModel.Job) error
}

func (m *MockJobStore) GetJob(ctx context.Context, jobId string) (jobModel.Job, bool) {
	//TODO implement me
	panic("implement me")
}

func (m *MockJobStore) DeleteJob(ctx context.Context, jobID string) {
	//TODO implement me
	panic("implement me")
}

func (m *MockJobStore) SaveJob(ctx context.Context, j jobModel.Job) error {
	if m.OnSaveJob != nil {
		return m.OnSaveJob(ctx, j)
	}
	return nil
}

// MockMessageStore handles chat history
type MockMessageStore struct {
	OnGetHistory func(ctx context.Context, chatId string) (error, []string)
	OnSaveChat   func(ctx context.Context, chatId string, payload jobModel.JobPayload) error
}

func (m *MockMessageStore) ValidateChatId(ctx context.Context, id string) bool {
	return true
}

func (m *MockMessageStore) InitNewChat(ctx context.Context, id string) error {
	return nil
}

func (m *MockMessageStore) GetMessageHistory(ctx context.Context, id string) (error, []string) {
	if m.OnGetHistory != nil {
		return m.OnGetHistory(ctx, id)
	}
	return nil, []string{}
}
func (m *MockMessageStore) TrySaveChat(ctx context.Context, id string, p jobModel.JobPayload) error {
	if m.OnSaveChat != nil {
		return m.OnSaveChat(ctx, id, p)
	}
	return nil
}

func TestWorkerPool_Flow(t *testing.T) {
	// 1. Setup
	jobSvc := &job.Service{
		JobChannel:        make(chan jobModel.Job, 10),
		DispatcherChannel: make(chan bool, 10),
		JobStore:          &MockJobStore{},
		MessageStore:      &MockMessageStore{},
	}
	mockRag := &MockRagService{}
	stopChan := make(chan bool)
	wg := &sync.WaitGroup{}

	InitServices(jobSvc, mockRag)
	InitWorkerPool(stopChan, wg)

	// Reset global state for test
	atomic.StoreInt64(&currentWorkerCount, 0)

	t.Run("Dispatcher creates worker on signal", func(t *testing.T) {
		// Signal dispatcher to create a worker
		jobSvc.DispatcherChannel <- true

		// Give it a millisecond to spawn
		time.Sleep(50 * time.Millisecond)

		count := atomic.LoadInt64(&currentWorkerCount)
		if count < 1 {
			t.Errorf("Expected at least 1 worker, got %d", count)
		}
	})

	t.Run("Worker processes a job", func(t *testing.T) {
		testJob := jobModel.Job{Id: "test-1"}
		jobSvc.JobChannel <- testJob

		// Wait for worker to pick up and process
		time.Sleep(50 * time.Millisecond)

		processed := atomic.LoadInt32(&mockRag.ProcessedCount)
		if processed != 1 {
			t.Errorf("Expected 1 job processed, got %d", processed)
		}
	})

	t.Run("Stop signal retires workers", func(t *testing.T) {
		// Send stop signal
		close(stopChan)

		// Wait for workers to exit
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Error("Workers did not stop within timeout")
		}
	})
}

func TestWorker_IdleTimeout(t *testing.T) {
	// Temporarily override config/globals for test
	atomic.StoreInt64(&currentWorkerCount, 0)
	atomic.StoreInt64(&minWorkerCount, 2) // Must be > 1 based on your logic
	logger = logger_i.NewLogger("TestWorkerPool")
	jobSvc := &job.Service{
		JobChannel: make(chan jobModel.Job),
	}
	InitServices(jobSvc, &MockRagService{})

	wg := &sync.WaitGroup{}
	stopChan := make(chan bool)
	workerWaitGroup = wg
	stopWorkerChannel = stopChan

	// Spawn 1 worker manually
	createWorker()
	time.Sleep(config.IdleWorkerTimeout)

	time.Sleep(100 * time.Millisecond)
	count := atomic.LoadInt64(&currentWorkerCount)
	if count != 0 {
		t.Errorf("Assertion Failed: Worker should have timed out and retired, but count is %d", count)
	}
}
