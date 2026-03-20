package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/akolanti/GoAPI/internal/adapter"
	"github.com/akolanti/GoAPI/internal/adapter/utils"
	"github.com/akolanti/GoAPI/internal/api"
	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/job"
	"github.com/akolanti/GoAPI/internal/mcpImpl"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

var logRH *logger_i.Logger

var service *job.Service

func InitHandler(jobService *job.Service) {
	service = jobService
}

func GetHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	return
}

// ChatHandler godoc
// @Summary      Start a new chat job
// @Description  Accepts a message, initializes a background processing job, and returns a job ID to track status.
// @Tags         Messaging
// @Accept       json
// @Produce      json
// @Param        request  body      api.ChatRequest      true  "Chat Message and optional Chat ID"
// @Success      202      {object}  api.InitJobResponse  "Job successfully created"
// @Failure      400      {object}  api.JobResponse      "Invalid request data or chat ID"
// @Router       /chat [post]
func ChatHandler(w http.ResponseWriter, request *http.Request) {

	if validateContext(request.Context()) {

		var requestData api.ChatRequest
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				logRH.Error("Couldn't close the Chat handler reader :", err)
			}
		}(request.Body)
		if err := json.NewDecoder(request.Body).Decode(&requestData); err != nil || !ValidateChatRequest(requestData) {

			logRH.Warn("Bad Chat Request: ", "error:", err, "request data:", requestData)
			WriteErrorResponse(w, http.StatusBadRequest, requestData.ChatID, "Bad Request")
			return
		}
		processNewJobData(request, w, requestData, "", "") //5 param method is ugly change this
		return
	}
	logRH.Warn("Invalid Context by request ", request.RemoteAddr)
}

// GetStatusHandler godoc
// @Summary      Get job status
// @Description  Retrieves the current status of a specific job using its ID.
// @Tags         Job Status
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Job ID "
// @Success      200  {object}  api.JobResponse "The current status of the job"
// @Success      200  {object}  api.JobResponse   "Successful retrieval of job status"
// @Failure      404  {object}  api.JobResponse   "Job not found (returns Error object within JobResponse)"
// @Router       /status/{id} [get]
func GetStatusHandler(w http.ResponseWriter, r *http.Request) {
	if validateContext(r.Context()) {
		//use chi get the url id
		idString := utils.GetChiURLParam(r, "id")
		result, isFound := validateId(idString, r.Context())

		logRH.Debug("Get Status Request:", "URL path", r.URL.Path)
		if !isFound {
			WriteErrorResponse(w, http.StatusNotFound, idString, "Job not found")

		}

		writeJsonResponse(w, http.StatusOK, adapter.ToAPIResponse(result))
	}
}

// PostIngestHandler handles the uploading of PDF or DOCX documents for RAG ingestion.
// @Summary      Upload a document for ingestion
// @Description  Receives a file via multipart/form-data, saves it to a temporary directory, and queues an ingestion job.
// @Tags         Ingestion
// @Accept       multipart/form-data
// @Produce      json
// @Param        document_name  formData  string  true  "The display name of the document"
// @Param        document       formData  file    true  "The PDF or DOCX file to upload"
// @Success      202  {object}  map[string]string "Accepted - returns job_id"
// @Failure      400  {object}  api.JobResponse "Bad Request - Missing fields or file too large"
// @Failure      500  {object}  api.JobResponse "Internal Server Error - Storage or Write Error"
// @Router       /ingest [post]
func PostIngestHandler(w http.ResponseWriter, r *http.Request) {
	if validateContext(r.Context()) {

		targetDir, errString := getTargetDirectory()

		if errString != "" {
			logRH.Error("Couldn't get target directory :", "err", errString)
			WriteErrorResponse(w, http.StatusInternalServerError, "", errString)
		}

		const maxUploadSize = 32 << 20 //32mb
		err := r.ParseMultipartForm(maxUploadSize)
		if err != nil {
			WriteErrorResponse(w, http.StatusBadRequest, "", "File too large or bad request")
			return
		}

		//process request
		docName := r.FormValue("document_name")
		if docName == "" {
			WriteErrorResponse(w, http.StatusBadRequest, "", "document_name is required")
			return
		}

		//get the document name the user uploads
		fileReader, fileMetadata, err := r.FormFile("document")
		if err != nil {
			WriteErrorResponse(w, http.StatusBadRequest, docName, "Could not retrieve file")
			return
		}
		defer fileReader.Close()

		filename := fmt.Sprintf("%d-%s", time.Now().UnixNano(), fileMetadata.Filename)
		tempFilePath := filepath.Join(targetDir, filename)
		destinationFileWriter, err := os.Create(tempFilePath)
		if err != nil {
			WriteErrorResponse(w, http.StatusInternalServerError, docName, "Storage error")
			return
		}
		defer destinationFileWriter.Close()

		if _, err := io.Copy(destinationFileWriter, fileReader); err != nil {
			WriteErrorResponse(w, http.StatusInternalServerError, docName, "Write error")
			return
		}
		processNewJobData(r, w, api.ChatRequest{}, filename, tempFilePath)
		return
	}
	logRH.Warn("Invalid Context by request ", r.RemoteAddr)
}

// MCPHandler godoc
// @Summary      Submit a stateless MCP query
// @Description  Accepts a question, runs tool-use via MCP and returns a job ID. This is stateless - each request is independent with no conversation history. Use /chat for multi-turn conversations.
// @Tags         MCP
// @Accept       json
// @Produce      json
// @Param        request  body      api.MCPRequest       true  "Question and optional request ID"
// @Success      202      {object}  api.InitJobResponse  "Job created - poll /mcp/status/{id}"
// @Failure      400      {object}  api.JobResponse      "Invalid request"
// @Router       /mcp [post]
func MCPHandler(w http.ResponseWriter, request *http.Request) {
	if validateContext(request.Context()) {
		var requestData api.MCPRequest
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				logRH.Error("Couldn't close the mcp reader :", err)
			}
		}(request.Body)
		if err := json.NewDecoder(request.Body).Decode(&requestData); err != nil || !ValidateMcpRequest(requestData) {
			logRH.Warn("Bad mcp Request: ", "error:", err, "request data:", requestData)
			WriteErrorResponse(w, http.StatusBadRequest, requestData.RequestId, "Bad Request")
			return
		}

		jobId := requestData.RequestId
		if jobId == "" {
			jobId = utils.GetNewUUID()
		}
		traceId := request.Context().Value(config.TRACE_ID_KEY).(string)

		mcpImpl.HandleRequest(request.Context(), requestData.Message, jobId, traceId)
		writeJsonResponse(w, http.StatusAccepted, adapter.ToInitJobResponse(jobId))
		return
	}
}

// MCPStatusHandler godoc
// @Summary      Get MCP job status
// @Description  Poll this endpoint to check the status of an MCP query. Returns the final answer when complete.
// @Tags         MCP
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Job ID from the /mcp response"
// @Success      200  {object}  api.JobResponse  "Current job status and result if complete"
// @Failure      404  {object}  api.JobResponse  "Job not found"
// @Router       /mcp/status/{id} [get]
func MCPStatusHandler(w http.ResponseWriter, r *http.Request) {
	if validateContext(r.Context()) {
		idString := utils.GetChiURLParam(r, "id")
		result, isFound := validateId(idString, r.Context())

		logRH.Debug("Get MCP Status Request:", "URL path", r.URL.Path)
		if !isFound {
			WriteErrorResponse(w, http.StatusNotFound, idString, "Job not found")
			return
		}
		writeJsonResponse(w, http.StatusOK, adapter.ToAPIResponse(result))
	}
}
