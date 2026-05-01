package bootstrap

import (
	"context"
	"log/slog"
	"sync"
)

type ServiceStartup interface {
	Run(ctx context.Context) error
	Name() string
}

type Service struct {
	logger *slog.Logger
}

func NewService(logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{logger: logger}
}

func (r *Service) Run(ctx context.Context, services []ServiceStartup) error {
	var (
		wg       sync.WaitGroup
		errOnce  sync.Once
		firstErr error
	)

	for _, service := range services {
		service := service
		wg.Add(1)

		go func() {
			defer wg.Done()

			r.logger.Info("starting", "component", service.Name())

			if err := service.Run(ctx); err != nil {
				errOnce.Do(func() {
					firstErr = err
				})
				r.logger.Error("component stopped with error", "component", service.Name(), "error", err)
				return
			}

			r.logger.Info("component stopped cleanly", "component", service.Name())
		}()
	}

	wg.Wait()
	return firstErr
}
