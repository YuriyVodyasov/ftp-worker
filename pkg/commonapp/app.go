package commonapp

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"ftp-worker/pkg/clogger"
	"ftp-worker/pkg/ctypes"
)

type StartStopper interface {
	Run(ctx context.Context)
	Shutdown(ctx context.Context) error
}

type GetterConfig interface {
	GetShutDownTimeout() ctypes.TimeDuration
	GetLoggerConfig() clogger.Config
}

type Config struct {
	Log             clogger.Config      `yaml:"log"`
	ShutDownTimeout ctypes.TimeDuration `yaml:"shut_down_timeout"`
	Ver             Version
}

func NewAndRunApp[T StartStopper, C GetterConfig](ctx context.Context, factory AppFactory[T, C], cfg *C) {
	l := clogger.New((*cfg).GetLoggerConfig())

	log.Logger = l.Logger

	app, err := factory.NewApp(ctx, cfg, l)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create application")
	}

	go func() {
		app.Run(ctx)
	}()

	<-ctx.Done()

	ctxShutdown, cancel := context.WithTimeout(context.Background(), time.Duration((*cfg).GetShutDownTimeout()))
	defer cancel()

	err = app.Shutdown(ctxShutdown)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to shutdown application")
	}

	log.Info().Msg("shutdown complete")
}

type AppFactory[T StartStopper, C any] interface {
	NewApp(ctx context.Context, cfg *C, logger clogger.Logger) (T, error)
}
