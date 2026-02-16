package handlers

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/akolanti/GoAPI/internal/api"
	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/domain/jobModel"
	"github.com/akolanti/GoAPI/internal/job"
	"github.com/akolanti/GoAPI/internal/metrics"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

var (
	handlerInstance *JobHandler //private singleton
	once            sync.Once
	logJH           *logger_i.Logger
)

type JobHandler struct {
	service *job.Service
}

func InitJobHandler(jobService *job.Service) {
	once.Do(func() {
		handlerInstance = &JobHandler{service: jobService}

		logJH = logger_i.NewLogger("JobHandler")
		logRH = logger_i.NewLogger("RequestHandler")
		logJH.Info("Starting job handler")
	})

}

func CreateNewJob(newJob newJobData) {
	logJH.With("traceId", newJob.traceId, "job id", newJob.id)
	logJH.Info("To create new job")
	handlerInstance.pushToJobChannel(newJob)
	if newJob.isNewChat {
		logJH.Info("Create new chat")
		handlerInstance.initNewChat(newJob.chatId, newJob.traceId)
	}
}

func GetJobStatus(id string, traceId string) (result jobModel.Job, isFound bool) {
	ctxC := context.WithValue(context.Background(), config.TRACE_ID_KEY, traceId)
	if handlerInstance != nil {
		return handlerInstance.service.JobStore.GetJob(ctxC, id)
	}
	return result, false
}

func ValidateChatRequest(chatReq api.ChatRequest) bool {
	if handlerInstance == nil {
		return false
	}
	logJH.Debug(" Validating chat id ", "chatId :", chatReq.ChatID)
	if chatReq.Message == "" {
		return false
	}
	if chatReq.ChatID == "" {
		return true
	}
	return handlerInstance.service.MessageStore.ValidateChatId(context.Background(), chatReq.ChatID)
}

// private methods
func (h *JobHandler) pushToJobChannel(newJob newJobData) {

	_job := jobModel.Job{}
	_job.Id = newJob.id
	_job.CreatedTime = time.Now()
	_job.TraceId = newJob.traceId
	_job.Status = jobModel.JobStatusQueued

	if newJob.isDocumentIngest {
		_job.CurrentStep = jobModel.IngestInit
		_job.JobType = jobModel.JobTypeIngest
		_job.JobPayload.IngestFileName = newJob.documentName
		_job.JobPayload.IngestURL = newJob.documentSource

	} else {
		_job.JobType = jobModel.JobTypeQuery
		_job.ChatId = newJob.chatId
		_job.JobPayload.Question = newJob.message
		_job.CurrentStep = jobModel.UserQueryInit
	}

	//metrics
	metrics.IncrementJobsInQueue()

	h.service.JobChannel <- _job //this is a blocking send to prevent the system from being overwhelmed
	logJH.Info("Created new job")

	//we will start a new worker every 10 requests - can also be configured
	// or
	//for performance - a new worker is added  for a document ingestion type job
	//ingestion involves batch processing which might take time - external system call
	//worker will be removed if it has idle time - so it should be ok
	//this also allows us to only keep 1 worker running at most times therefore cutting resource spend

	accurateCount := atomic.AddInt64(&h.service.RequestCount, 1) //after sending a request increment counter
	if accurateCount%config.RequestsPerNewWorkerCount == 0 || _job.JobType == jobModel.JobTypeIngest {
		metrics.StartDispatcherSignalCount() //metrics
		logJH.Debug("Worker count ", accurateCount)
		h.service.DispatcherChannel <- true
	}
}

func (h *JobHandler) initNewChat(chatId string, traceId string) {
	ctxC := context.WithValue(context.Background(), config.TRACE_ID_KEY, traceId)
	err := h.service.MessageStore.InitNewChat(ctxC, chatId)
	if err != nil {
		logJH.Error("Error initiating new chat", chatId, err)
		return
	}
}
