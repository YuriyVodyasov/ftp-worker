package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"ftp-worker/pkg/config"

	"ftp-worker/internal/store/engine"
	"ftp-worker/internal/store/engine/pg"
	"ftp-worker/internal/store/service"
)

func New(ctx context.Context, cfg Config) (*service.Store, error) {
	var engine engine.AuxStore

	switch strings.ToLower(cfg.Engine) {
	case "postgres":
		dbCfg, err := config.Get[pg.PGPoolCfg](cfg.DBCfg)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get DB config")
		}

		engine, err = pg.New(ctx, dbCfg, cfg.ErrorProcessing)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create PG engine")
		}
	default:
		return nil, fmt.Errorf("unsupported store engine: %s", cfg.Engine)
	}

	return service.New(engine), nil
}

type Config struct {
	Engine          string `yaml:"engine"`
	ErrorProcessing bool   `yaml:"error_processing"`
	DBCfg           string `yaml:"db_cfg"`
}
