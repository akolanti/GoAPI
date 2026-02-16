package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/akolanti/GoAPI/internal/adapter"
	"github.com/akolanti/GoAPI/internal/adapter/utils"
	"github.com/akolanti/GoAPI/internal/api"
	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/domain/jobModel"
)

func writeJsonResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Log the error but can't send a clean status code now
		logRH.Error("Error encoding response: %v", err)
	}
}

func validateId(id string, traceId string) (result jobModel.Job, isFound bool) {
	if id == "" {
		logRH.Warn("Empty Job ID")
		return jobModel.Job{}, false
	}
	return GetJobStatus(id, traceId)
}

func validateContext(ctx context.Context) bool {
	logRH.With("traceId:", ctx.Value(config.TRACE_ID_KEY).(string))
	if ctx.Err() != nil {
		logRH.Warn("context error", ctx.Err())
		return false
	}

	select {
	case <-ctx.Done():
		logRH.Warn("context cancelled")
		return false
	default:
		return true

	}
}

func WriteErrorResponse(w http.ResponseWriter, httpCode int, id string, error string) {
	writeJsonResponse(w, httpCode, adapter.BadRequest(id, error, httpCode))
}

func getTargetDirectory() (string, string) {
	root, err := os.Getwd()
	if err != nil {
		return "", "Storage Error"
	}

	targetDir := filepath.Join(root, "temporary_data")
	if err := os.MkdirAll(targetDir, 0750); err != nil {
		return "", "Storage Error"
	}
	return targetDir, ""
}

func processNewJobData(request *http.Request, w http.ResponseWriter, requestData api.ChatRequest, docName string, docPath string) {
	chatID := ""
	message := ""
	isNewChat := false

	//if documentName is empty then it's a chat request
	isChatRequest := docName == "" && docPath == ""

	if isChatRequest {
		chatID = requestData.ChatID
		if chatID == "" {
			chatID = utils.GetNewUUID()
			logRH.Debug(" New Chat request : ", "chatID:", chatID)
			isNewChat = true
		}
		message = requestData.Message
	}

	newJob := newJobData{
		id:               utils.GetNewUUID(),
		chatId:           chatID,
		message:          message,
		isNewChat:        isNewChat,
		traceId:          request.Context().Value(config.TRACE_ID_KEY).(string),
		documentName:     docName,
		documentSource:   docPath,
		isDocumentIngest: !isChatRequest,
	}
	CreateNewJob(newJob)
	res := adapter.ToInitJobResponse(newJob.id)
	writeJsonResponse(w, http.StatusAccepted, res)

}
