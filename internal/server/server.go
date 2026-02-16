package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"sync"

	"github.com/akolanti/GoAPI/internal/adapter/utils"
	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/middleware"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

var (
	server  *http.Server
	_logger *logger_i.Logger
)

type ShutdownParams struct {
	GracefulShutdown chan os.Signal
	StopExecution    chan bool
	WorkerStop       chan bool
	Group            *sync.WaitGroup
	CloseServices    context.CancelFunc
}

func CreateServer(listenAddr string) {
	_logger = logger_i.NewLogger("Server")

	r := utils.GetRouter()

	r.Router.Post("/chat", middleware.ChatHandler)
	r.Router.Get("/status/{id}", middleware.GetStatusHandler)
	r.Router.Post("/ingest", middleware.PostIngestHandler)
	server = &http.Server{
		Addr:         listenAddr,
		Handler:      r.Router,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}

	_logger.Info("Server is listening at", "address", listenAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		_logger.Error("Server crashed", "error :", err.Error(), "addr", listenAddr)
	}
}

func ShutDownHandler(shutdownParams ShutdownParams) {
	state := <-shutdownParams.GracefulShutdown
	println("\nServer is shutting down", state)

	ctx, cancel := context.WithTimeout(context.Background(), config.ShutdownContextTimeout)
	defer cancel()

	done := make(chan struct{})

	go func() {
		server.SetKeepAlivesEnabled(false)

		if err := server.Shutdown(ctx); err != nil {
			_logger.Error("Could not shutdown gracefully: %s", err)
		}

		//close workers
		close(shutdownParams.WorkerStop)
		shutdownParams.Group.Wait()
		shutdownParams.CloseServices()
		close(shutdownParams.StopExecution)
		close(done)
	}()

	select {
	case <-done:
		_logger.Info("Gracefully is shutting down")
	case <-ctx.Done():
		_logger.Info("Force Shut down")
		os.Exit(1)
	}
}
