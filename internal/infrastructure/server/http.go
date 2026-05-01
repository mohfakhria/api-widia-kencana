package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/config"
)

type HTTPServer struct {
	server *http.Server
}

func NewHTTPServer(cfg config.Config, handler *gin.Engine) *HTTPServer {
	return &HTTPServer{
		server: &http.Server{
			Addr:              cfg.Address(),
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
}

func (s *HTTPServer) Name() string {
	return "http-server"
}

func (s *HTTPServer) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		errCh <- s.server.Shutdown(shutdownCtx)
	}()

	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return <-errCh
}
