package adapter

import (
	"fmt"
	"time"

	"github.com/akolanti/GoAPI/internal/api"
	"github.com/akolanti/GoAPI/internal/domain/jobModel"
)

func ToInitJobResponse(id string) api.InitJobResponse {
	return api.InitJobResponse{
		Id:        id,
		StatusURL: fmt.Sprintf("status/%s", id), //pass "status/job.Id"
	}
}

func ToAPIResponse(job jobModel.Job) api.JobResponse {

	var errorPtr *api.JobOutgoingError
	if job.Error.Message != "" || job.Error.Code != 0 {
		errorPtr = &api.JobOutgoingError{
			Code:    job.Error.Code,
			Message: job.Error.Message,
			Retry:   job.Error.Retry,
		}
	}

	result := api.Result{
		Status:           string(job.Status),
		ExternalResponse: ToRAGExternalStatus(job.JobPayload),
	}

	return api.JobResponse{
		Id:        job.Id,
		ChatId:    job.ChatId,
		StartTime: job.CreatedTime,
		EndTime:   job.EndTime,
		Error:     errorPtr,
		Result:    result,
	}
}

func ToRAGExternalStatus(ragData jobModel.JobPayload) *api.Response {
	if ragData.Answer == "" && len(ragData.Sources) == 0 {
		return nil
	}

	return &api.Response{
		Question: ragData.Question,
		Answer:   ragData.Answer,
		Sources:  ragData.Sources,
	}
}

func BadRequest(id string, error string, code int) api.JobResponse {
	return api.JobResponse{
		Id:        id,
		ChatId:    "",
		StartTime: time.Time{},
		EndTime:   time.Time{},
		Result: api.Result{
			Status:           string(api.JobStatusError),
			ExternalResponse: ToRAGExternalStatus(jobModel.JobPayload{}),
		},
		Error: &api.JobOutgoingError{
			Code:    code,
			Message: error,
			Retry:   false,
		},
	}
}

