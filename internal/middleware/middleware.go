package middleware

import (
	"net/http"
	"strconv"

	"github.com/akolanti/GoAPI/internal/handlers"
	"github.com/akolanti/GoAPI/internal/metrics"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

type requestResponseStruct struct {
	writer     http.ResponseWriter
	req        *http.Request
	badRequest failureStruct
	logger     *logger_i.Logger
}

type failureStruct struct {
	isBadRequest bool
	httpCode     int
	errorMessage string
	id           string
}

var GetHandler = Wrap(handlers.GetHandler)

var ChatHandler = Wrap(handlers.ChatHandler)
var GetStatusHandler = Wrap(handlers.GetStatusHandler)
var PostIngestHandler = Wrap(handlers.PostIngestHandler)

func Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := &metrics.HttpStatusRecorder{ResponseWriter: w, Status: 200} //metrics
		re := processRequest(requestResponseStruct{req: r, writer: rec})

		if re.badRequest.isBadRequest {
			handleBadRequest(re)
			return
		}
		next(rec, re.req)

		metrics.HttpRequestsTotal.WithLabelValues(r.URL.Path, strconv.Itoa(rec.Status)).Inc() //metrics
	}
}
func processRequest(re requestResponseStruct) requestResponseStruct {
	re.logger = logger_i.NewLogger("middleware")
	re.logger.Info("New request received")
	//TODO:make this cleaner
	re = injectTrace(re)
	re = authenticate(re)
	if re.badRequest.isBadRequest {
		handleBadRequest(re)
		return re //stop if auth fails
	}
	//re = rateLimiter(re)
	//if re.badRequest.isBadRequest {
	//	handleBadRequest(re)
	//	return re //stop here if rate limit fails
	//}

	return re
}
