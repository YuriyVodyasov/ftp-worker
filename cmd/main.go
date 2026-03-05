package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"

	"ftp-worker/pkg/commonapp"
	"ftp-worker/pkg/config"
	"ftp-worker/pkg/options"

	"ftp-worker/internal/application"
)

//nolint:gochecknoglobals // for LDFLAGS
var (
	name    string
	version string
	commit  string
	date    string
	dirty   string
)

func main() {
	opts := options.Get()

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGKILL)

	defer stop()

	cfg, err := config.Get[application.Config](opts.ConfigFile)
	if err != nil {
		log.Fatal().Err(err).Msg("can't get config")
	}

	cfg.Ver = commonapp.Version{
		Name:   name,
		Ver:    version,
		Commit: commit,
		Date:   date,
		Dirty:  dirty,
	}

	factory := application.AppFactory{}

	commonapp.NewAndRunApp(ctx, factory, cfg)
}
