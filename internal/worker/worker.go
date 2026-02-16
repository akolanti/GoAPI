package worker

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/job"
	"github.com/akolanti/GoAPI/internal/metrics"
	"github.com/akolanti/GoAPI/internal/rag"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

var (
	_jobService        *job.Service
	stopWorkerChannel  chan bool
	workerWaitGroup    *sync.WaitGroup
	dispatcherChannel  chan bool
	currentWorkerCount int64
	logger             *logger_i.Logger
	_ragService        rag.Service
	minWorkerCount     = config.MinWorkerCount
)

func InitServices(jobService *job.Service, ragService rag.Service) {
	_jobService = jobService
	_ragService = ragService
	dispatcherChannel = jobService.DispatcherChannel
}

func InitWorkerPool(stopWorkerChan chan bool, waitGroup *sync.WaitGroup) {
	stopWorkerChannel = stopWorkerChan
	workerWaitGroup = waitGroup
	logger = logger_i.NewLogger("WorkerPool")
	logger.Info("Initializing worker pool")
	go dispatcher()
}

func dispatcher() {
	createWorker()
	logger.Info("Dispatcher started")
	for range dispatcherChannel {
		if atomic.LoadInt64(&currentWorkerCount) < config.MaxWorkerCount {
			logger.Info("Creating new worker", "WorkerCount :", currentWorkerCount)
			createWorker()
		}
	}
}

func createWorker() {
	workerWaitGroup.Add(1)
	go worker()
	atomic.AddInt64(&currentWorkerCount, 1)
	metrics.IncrementActiveWorkerCount()
	logger.Info("Created new worker")
}

func worker() {
	for {
		select {
		case currentJob := <-_jobService.JobChannel:
			executeJob(currentJob)
			metrics.DecrementJobsInQueue()

		case <-stopWorkerChannel:
			removeWorker("Stop worker signal received")

			return

		case <-time.After(config.IdleWorkerTimeout):
			// Worker was idle for too long, decrement counter and retire
			if atomic.LoadInt64(&minWorkerCount) > 1 {
				removeWorker(" Idle worker timeout - Removed worker")
				return
			}
		}
	}
}
