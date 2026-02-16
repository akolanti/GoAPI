package worker

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/akolanti/GoAPI/internal/config"
	jobmodel "github.com/akolanti/GoAPI/internal/domain/jobModel"
	"github.com/akolanti/GoAPI/internal/metrics"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

func executeJob(job jobmodel.Job) {
	start := time.Now()
	defer func() {
		// Record total time at the end
		metrics.CaptureJobMetrics(string(job.Status), time.Since(start))
	}()
	ctxTrace := context.WithValue(context.Background(), config.TRACE_ID_KEY, job.TraceId)
	ctx, cancel := context.WithTimeout(ctxTrace, 60*time.Second)
	defer cancel()
	logger.With("trace Id ", job.TraceId)
	logger.Debug("Processing job:", "job Id:", job.Id)

	saveJobState(ctx, job, jobmodel.JobStatusRunning)

	if job.JobType == jobmodel.JobTypeIngest {
		job.CurrentStep = jobmodel.IngestProcessing
		job = ingestDocument(job, ctx, logger)

	} else {
		job.CurrentStep = jobmodel.RedisCall
		job = processQuery(job, ctx, logger)
		if job.Status != jobmodel.JobStatusError {
			if err := _jobService.MessageStore.TrySaveChat(ctx, job.ChatId, job.JobPayload); err != nil {
				logger.Error("Failed to save chat history", "err", err)
			}
		}
	}

	job.EndTime = time.Now()
	saveJobState(ctx, job, jobmodel.JobStatusComplete)
}

func removeWorker(reason string) {

	workerWaitGroup.Done()
	atomic.AddInt64(&currentWorkerCount, -1)
	logger.Info("Removed worker ", "reason", reason, "workerCount", currentWorkerCount)
	metrics.DecrementActiveWorkerCount()

}

func ingestDocument(job jobmodel.Job, ctx context.Context, logger *logger_i.Logger) jobmodel.Job {
	t := _ragService.IngestDocument(ctx, job)
	print(t.CurrentStep)
	return job
}

func processQuery(job jobmodel.Job, ctx context.Context, logger *logger_i.Logger) jobmodel.Job {
	err, messageHistory := _jobService.MessageStore.GetMessageHistory(ctx, job.ChatId)
	if err != nil {
		logger.Error("Failed to get message history", "err", err)
	}
	job = _ragService.ProcessRequest(ctx, job, messageHistory)
	return job
}

func saveJobState(ctx context.Context, job jobmodel.Job, jobStatus jobmodel.JobStatus) {
	job.Status = jobStatus
	if err := _jobService.JobStore.SaveJob(ctx, job); err != nil {
		logger.Error("Failed to update status in Redis", "err", err)
	}
}
