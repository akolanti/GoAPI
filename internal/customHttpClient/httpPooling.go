package customHttpClient

import (
	"net/http"

	"github.com/akolanti/GoAPI/internal/config"
)

//TODO: make qdrant/llm/embedder reuse connections to avoid latency

var customTransport = &http.Transport{
	MaxIdleConns:        config.MaxIdleConns,
	MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
	IdleConnTimeout:     config.IdleConnTimeout,
}
