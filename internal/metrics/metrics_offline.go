//go:build offline

package metrics

import (
	"net/http"
	"time"
)

// noopCounter satisfies the .WithLabelValues(...).Inc() call chain
type noopCounter struct{}

func (noopCounter) Inc()                          {}
func (noopCounter) WithLabelValues(...string) noopCounter { return noopCounter{} }

var HttpRequestsTotal = noopCounter{}

type HttpStatusRecorder struct {
	http.ResponseWriter
	Status int
}

func (r *HttpStatusRecorder) WriteHeader(code int) {
	r.Status = code
	r.ResponseWriter.WriteHeader(code)
}

func IncrementJobsInQueue()    {}
func DecrementJobsInQueue()    {}
func StartDispatcherSignalCount() {}
func IncrementActiveWorkerCount() {}
func DecrementActiveWorkerCount() {}
func CaptureExecutionMetrics(_ string, _ time.Duration) {}
func CaptureJobMetrics(_ string, _ time.Duration)       {}
