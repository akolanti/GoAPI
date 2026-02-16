// @title           Chat RAG API
// @version         1.0
// @description     This API handles asynchronous chat RAG
// @termsOfService  http://swagger.io/terms/

// @contact.name    me lol
// @contact.url
// @contact.email

// @license.name    Apache 2.0
// @license.url     http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:3000
// @BasePath  /
// @schemes   http https
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/data/store"
	jobmodel "github.com/akolanti/GoAPI/internal/domain/jobModel"
	"github.com/akolanti/GoAPI/internal/handlers"
	"github.com/akolanti/GoAPI/internal/job"
	"github.com/akolanti/GoAPI/internal/rag"
	"github.com/akolanti/GoAPI/internal/rag/embedding/googleEmbedding"
	"github.com/akolanti/GoAPI/internal/rag/llm/gemini"
	"github.com/akolanti/GoAPI/internal/rag/vectorDB/qdrantDB"
	"github.com/akolanti/GoAPI/internal/server"
	"github.com/akolanti/GoAPI/internal/worker"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

var (
	listenAddr        string
	requestCount      int64
	stopWorkerChannel chan bool
	workerWaitGroup   sync.WaitGroup
)

func main() {

	logger_i.Init()
	var logger = logger_i.NewLogger("main")

	//config
	flag.StringVar(&listenAddr, "listen-addr", config.ServerListenAddr, "server listen address")
	flag.Parse()

	//init buffered job channel
	jobChannel := make(chan jobmodel.Job, config.BufferLimit)
	dispatcherChannel := make(chan bool, 1)
	stopWorkerChannel = make(chan bool, 1)

	serviceContext, closeExternalServices := context.WithCancel(context.Background())
	defer closeExternalServices()

	//init job service and job store
	serviceConfig := job.ServiceConfig{
		JobChannel:        jobChannel,
		RequestCount:      requestCount,
		DispatcherChannel: dispatcherChannel,
		JobStore:          store.GetRedisJobStore(serviceContext),
		MessageStore:      store.GetRedisMessageStore(serviceContext),
	}
	logger.Info("Starting job service")

	if serviceConfig.JobStore == nil || serviceConfig.MessageStore == nil {
		logger.Error("Redis stores are offline")
		serviceConfig.JobStore = store.InitInMemoryJobStore()
		serviceConfig.MessageStore = store.InitMessageStore()
	}
	service := job.InitJobService(serviceConfig)

	vectorDB := qdrantDB.GetQuadrantClient(serviceContext)
	embeddingService := googleEmbedding.GetGoogleEmbeddingClient(serviceContext, config.GoogleEmbeddingModel, config.GoogleEmbeddingAPIKey)
	llmProvider := gemini.GetGeminiClient(serviceContext, config.GoogleEmbeddingAPIKey, config.GeminiModelName)

	if vectorDB == nil || embeddingService == nil || llmProvider == nil {
		logger.Error("One or more external services failed to initialize. Shutting down.")
		logger.Debug("Available services : ", "VectorDB", vectorDB != nil, "EmbeddingService", embeddingService != nil, "LLMProvider", llmProvider != nil)
		return
	}

	ragService := rag.NewService(vectorDB, llmProvider, embeddingService)

	handlers.InitJobHandler(service)

	//init worker pool
	worker.InitServices(service, ragService)
	worker.InitWorkerPool(stopWorkerChannel, &workerWaitGroup)

	//server handling
	gracefulShutdown := make(chan os.Signal, 1)
	signal.Notify(gracefulShutdown, syscall.SIGINT, syscall.SIGTERM)
	stopExecution := make(chan bool, 1)

	shutdownParams := server.ShutdownParams{
		GracefulShutdown: gracefulShutdown,
		StopExecution:    stopExecution,
		WorkerStop:       stopWorkerChannel,
		Group:            &workerWaitGroup,
		CloseServices:    closeExternalServices,
	}
	go server.ShutDownHandler(shutdownParams)
	go server.CreateServer(listenAddr)

	<-stopExecution
	logger.Info("Server stopped")
}
