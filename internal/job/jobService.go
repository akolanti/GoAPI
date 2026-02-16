package job

import (
	"github.com/akolanti/GoAPI/internal/domain/jobModel"
)

type Service struct {
	JobChannel        chan jobModel.Job
	RequestCount      int64
	DispatcherChannel chan bool
	JobStore          jobModel.JobStore
	MessageStore      jobModel.MessageStore
}

type ServiceConfig struct {
	JobChannel        chan jobModel.Job
	RequestCount      int64
	DispatcherChannel chan bool
	JobStore          jobModel.JobStore
	MessageStore      jobModel.MessageStore
}

func InitJobService(cfg ServiceConfig) *Service {
	return &Service{
		JobChannel:        cfg.JobChannel,
		RequestCount:      cfg.RequestCount,
		DispatcherChannel: cfg.DispatcherChannel,
		JobStore:          cfg.JobStore,
		MessageStore:      cfg.MessageStore,
	}
}
