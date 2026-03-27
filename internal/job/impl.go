package job

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/domain/jobModel"
	"github.com/akolanti/GoAPI/internal/metrics"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

var logJH = logger_i.NewLogger("JobHandler")

func (s *Service) CreateJob(newJob CreateJobParams) {
	logJH.With("traceId", newJob.TraceID, "job id", newJob.ID)
	logJH.Info("To create new job")
	s.pushToJobChannel(newJob)
	if newJob.IsNewChat {
		logJH.Info("Create new chat")
		s.initNewChat(newJob.ChatID, newJob.TraceID)
	}
}

func (s *Service) GetJobStatus(id string, ctx context.Context) (result jobModel.Job, isFound bool) {
	if s != nil {
		return s.JobStore.GetJob(ctx, id)
	}
	return result, false
}

// private methods
func (s *Service) pushToJobChannel(newJob CreateJobParams) {

	_job := jobModel.Job{}
	_job.Id = newJob.ID
	_job.CreatedTime = time.Now()
	_job.TraceId = newJob.TraceID
	_job.Status = jobModel.JobStatusQueued

	if newJob.IsDocumentIngest {
		_job.CurrentStep = jobModel.IngestInit
		_job.JobType = jobModel.JobTypeIngest
		_job.JobPayload.IngestFileName = newJob.DocumentName
		_job.JobPayload.IngestURL = newJob.DocumentSource

	} else {
		_job.JobType = jobModel.JobTypeQuery
		_job.Status = jobModel.JobStatusQueued
		_job.ChatId = newJob.ChatID
		_job.JobPayload.Question = newJob.Message
		_job.CurrentStep = jobModel.UserQueryInit

		if newJob.IsMCPCall {
			_job.JobType = jobModel.JobTypeMCP
		} else {
			_job.JobType = jobModel.JobTypeQuery
		}
	}

	//metrics
	metrics.IncrementJobsInQueue()

	s.JobChannel <- _job //this is a blocking send to prevent the system from being overwhelmed
	logJH.Info("Created new job")

	//we will start a new worker every 10 requests - can also be configured
	// or
	//for performance - a new worker is added  for a document ingestion type job
	//ingestion involves batch processing which might take time - external system call
	//worker will be removed if it has idle time - so it should be ok
	//this also allows us to only keep 1 worker running at most times therefore cutting resource spend

	accurateCount := atomic.AddInt64(&s.RequestCount, 1) //after sending a request increment counter
	if accurateCount%config.RequestsPerNewWorkerCount == 0 || _job.JobType == jobModel.JobTypeIngest {
		metrics.StartDispatcherSignalCount() //metrics
		logJH.Debug("Worker count ", accurateCount)
		s.DispatcherChannel <- true
	}
}

func (s *Service) initNewChat(chatId string, traceId string) {
	ctxC := context.WithValue(context.Background(), config.TRACE_ID_KEY, traceId)
	err := s.MessageStore.InitNewChat(ctxC, chatId)
	if err != nil {
		logJH.Error("Error initiating new chat", chatId, err)
		return
	}
}

