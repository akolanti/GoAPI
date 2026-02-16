package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var HttpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "http_requests_total",
	Help: "Total number of requests labelled by path and status",
}, []string{"path", "status"})

var countJobsInQueue = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "count_jobs_in_queue",
	Help: "Number of jobs in queue",
})

var dispatcherSignalCount = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "dispatcher_signal_count",
	Help: "How often the dispatcher has signaled to start worker",
})

var activeWorkerCount = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "active_worker_count",
	Help: "Number of active workers",
})

type HttpStatusRecorder struct {
	http.ResponseWriter
	Status int
}

func (r *HttpStatusRecorder) CaptureWriteHeaderMetrics(code int) {
	r.Status = code
	r.ResponseWriter.WriteHeader(code)
}

func IncrementJobsInQueue() {
	countJobsInQueue.Inc()
}

func DecrementJobsInQueue() {
	countJobsInQueue.Dec()
}

func StartDispatcherSignalCount() {
	dispatcherSignalCount.Inc()
}

func IncrementActiveWorkerCount() {
	activeWorkerCount.Inc()
}
func DecrementActiveWorkerCount() {
	activeWorkerCount.Dec()
}

var requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "process_request_duration_seconds",
	Help:    "Total time spent in ProcessRequest.",
	Buckets: []float64{.1, .5, 1, 2, 5, 10, 30},
}, []string{"status"})

var dependencyLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "dependency_latency_seconds",
	Help:    "Latency of external service calls.",
	Buckets: []float64{.05, .1, .25, .5, 1, 2, 5, 10},
}, []string{"service"})

func CaptureExecutionMetrics(label string, timeElapsed time.Duration) {
	//dependencyLatency.WithLabelValues(label).Observe(time.Since(timeElapsed).Seconds())
	dependencyLatency.WithLabelValues(label).Observe(timeElapsed.Seconds())
}

func CaptureJobMetrics(label string, timeElapsed time.Duration) {
	requestDuration.WithLabelValues(label).Observe(timeElapsed.Seconds())
}
