package application

import (
	"context"

	"github.com/pkg/errors"

	"ftp-worker/pkg/cftp"
	"ftp-worker/pkg/clogger"
	"ftp-worker/pkg/commonapp"
	"ftp-worker/pkg/cryptoservice"
	"ftp-worker/pkg/ctypes"
	"ftp-worker/pkg/eapi"
	"ftp-worker/pkg/scheduler"

	"ftp-worker/internal/store"
	"ftp-worker/internal/store/service"
	"ftp-worker/internal/worker"
)

type App struct {
	scheduler *scheduler.Scheduler
	worker    *worker.Worker
}

func (i App) Run(ctx context.Context) {
	i.worker.Do(ctx)
	i.scheduler.Run(ctx)
}

func (i App) Shutdown(ctx context.Context) error {
	i.scheduler.Shutdown(ctx)
	i.worker.Shutdown(ctx)

	err := i.worker.StoreDB.Close(ctx)

	return errors.Wrap(err, "failed to close store")
}

type AppFactory struct{}

func (AppFactory) NewApp(ctx context.Context, cfg *Config, logger clogger.Logger) (App, error) {
	engineDB, err := store.New(ctx, cfg.Store)
	if err != nil {
		return App{}, errors.Wrap(err, "failed to make data store engine")
	}

	storeDB := service.New(engineDB)

	eapi, err := eapi.New(cfg.Eapi)
	if err != nil {
		return App{}, errors.Wrap(err, "failed to create eapi client")
	}

	crypto, err := cryptoservice.New(cfg.Crypto)
	if err != nil {

		return App{}, errors.Wrap(err, "failed to create crypto client")
	}

	worker := &worker.Worker{StoreDB: storeDB, Eapi: eapi, Crypto: crypto, FTPCfg: &cfg.FTP, FTPPoolCfg: &cfg.FTPPool, Log: &logger}

	scheduler := scheduler.New(cfg.GetShutDownTimeout().Duration(), worker)

	return App{scheduler: scheduler, worker: worker}, nil
}

type Config struct {
	commonapp.Config `yaml:",inline"`
	Store            store.Config         `yaml:"store"`
	Eapi             eapi.Config          `yaml:"eapi"`
	Crypto           cryptoservice.Config `yaml:"crypto"`
	FTP              cftp.Config          `yaml:"ftp"`
	FTPPool          cftp.PoolConfig      `yaml:"ftp_pool"`
}

func (c Config) GetShutDownTimeout() ctypes.TimeDuration {
	return c.ShutDownTimeout
}

func (c Config) GetLoggerConfig() clogger.Config {
	return c.Log
}
