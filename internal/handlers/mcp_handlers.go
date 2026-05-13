//go:build !offline

package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/akolanti/GoAPI/internal/adapter"
	"github.com/akolanti/GoAPI/internal/adapter/utils"
	"github.com/akolanti/GoAPI/internal/api"
	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/mcpImpl"
)

// MCPHandler godoc
// @Summary      Submit a stateless MCP query
// @Description  Accepts a question, runs tool-use via MCP and returns a job ID. This is stateless - each request is independent with no conversation history. Use /chat for multi-turn conversations.
// @Tags         MCP
// @Accept       json
// @Produce      json
// @Param        request  body      api.MCPRequest       true  "Question"
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
			WriteErrorResponse(w, http.StatusBadRequest, "", "Bad Request")
			return
		}

		jobId := utils.GetNewUUID()
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
