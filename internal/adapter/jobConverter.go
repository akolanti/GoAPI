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
		Sources:  formatSourcesForAPI(ragData.Sources),
	}
}

// formatSourcesForAPI extracts doc_name and page_num from internal metadata
// and returns clean, deduplicated source references for the API response.
func formatSourcesForAPI(raw []string) []string {
	type sourceKey struct{ doc, page string }
	seen := make(map[sourceKey]bool)
	var clean []string

	var currentDoc, currentPage string
	for _, entry := range raw {
		switch {
		case len(entry) > 9 && entry[:9] == "page_num:":
			currentPage = entry[9:]
		case len(entry) > 9 && entry[:9] == "doc_name:":
			currentDoc = entry[9:]
		}
		// Each group ends with score, flush when we have both
		if len(entry) > 6 && entry[:6] == "score:" && currentDoc != "" {
			key := sourceKey{currentDoc, currentPage}
			if !seen[key] {
				seen[key] = true
				clean = append(clean, fmt.Sprintf("%s (page %s)", currentDoc, currentPage))
			}
			currentDoc = ""
			currentPage = ""
		}
	}

	if len(clean) == 0 {
		return raw
	}
	return clean
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
