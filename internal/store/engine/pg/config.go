package pg

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/pkg/errors"

	"ftp-worker/pkg/ctls"
)

type Address struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type Credentials struct {
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type Config struct {
	Address         `yaml:",inline"`
	Credentials     `yaml:",inline"`
	DBName          string      `yaml:"dbname"`
	SSLmode         string      `yaml:"sslmode"`
	TLS             ctls.Config `yaml:"tls"`
	Migration       bool        `yaml:"migration"` // Should migrations be run when starting with sidecar?
	MigrationFolder string      `yaml:"migration_folder"`
}

func (cfg Config) BuildConnectionURL() (string, error) {
	connURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		cfg.User,
		url.QueryEscape(cfg.Password),
		cfg.Host,
		cfg.Port,
		cfg.DBName)

	switch strings.ToLower(cfg.SSLmode) {
	case "disable":
		connURL = fmt.Sprintf("%s?sslmode=disable", connURL)
	case "require":
		connURL = fmt.Sprintf("%s?sslmode=require", connURL)
	case "verify-ca":
		connURL = fmt.Sprintf("%s?sslmode=verify-ca&sslrootcert=%s", connURL, cfg.TLS.CACert)
	case "verify-full":
		connURL = fmt.Sprintf("%s?sslmode=verify-full&sslrootcert=%s&sslcert=%s&sslkey=%s", connURL, cfg.TLS.CACert, cfg.TLS.ClientCert, cfg.TLS.ClientKey)
	default:
		return "", fmt.Errorf("invalid sslmode: %s", cfg.SSLmode)
	}

	return connURL, nil
}

type Pool struct {
	MaxConns              int    `yaml:"max_conns"`                 // greater than 0 (default 4)
	MinConns              int    `yaml:"min_conns"`                 // 0 or greater (default 0)
	MaxConnLifetime       string `yaml:"max_conn_lifetime"`         // duration string (default 1 hour)
	MaxConnIdleTime       string `yaml:"max_conn_idle_time"`        // duration string (default 30 minutes)
	HealthCheckPeriod     string `yaml:"health_check_period"`       // duration string (default 1 minute)
	MaxConnIdleTimeJitter string `yaml:"max_conn_idle_time_jitter"` // duration string (default 0 seconds)
}

func (cfg Pool) BuildPoolConfig() (string, error) {
	poolConfig := strings.Builder{}

	if cfg.MaxConns > 0 {
		_, err := fmt.Fprintf(&poolConfig, "&pool_max_conns=%d ", cfg.MaxConns)
		if err != nil {
			return "", errors.Wrap(err, "format pool_max_conns")
		}
	}

	if cfg.MinConns > 0 {
		_, err := fmt.Fprintf(&poolConfig, "&pool_min_conns=%d ", cfg.MinConns)
		if err != nil {
			return "", errors.Wrap(err, "format pool_min_conns")
		}
	}

	if cfg.MaxConnLifetime != "" {
		_, err := fmt.Fprintf(&poolConfig, "&pool_max_conn_lifetime=%s ", cfg.MaxConnLifetime)
		if err != nil {
			return "", errors.Wrap(err, "format pool_max_conn_lifetime")
		}
	}

	if cfg.MaxConnIdleTime != "" {
		_, err := fmt.Fprintf(&poolConfig, "&pool_max_conn_idle_time=%s ", cfg.MaxConnIdleTime)
		if err != nil {
			return "", errors.Wrap(err, "format pool_max_conn_idle_time")
		}
	}

	if cfg.HealthCheckPeriod != "" {
		_, err := fmt.Fprintf(&poolConfig, "&pool_health_check_period=%s ", cfg.HealthCheckPeriod)
		if err != nil {
			return "", errors.Wrap(err, "format pool_health_check_period")
		}
	}

	if cfg.MaxConnIdleTimeJitter != "" {
		_, err := fmt.Fprintf(&poolConfig, "&pool_max_conn_idle_time_jitter=%s ", cfg.MaxConnIdleTimeJitter)
		if err != nil {
			return "", errors.Wrap(err, "format pool_max_conn_idle_time_jitter")
		}
	}

	return strings.TrimSpace(poolConfig.String()), nil
}

type PGPoolCfg struct {
	PGCfg Config `yaml:"pg"`
	Pool  Pool   `yaml:"pool"`
}

func (cfg PGPoolCfg) BuildConnectionURL() (string, error) {
	connURL, err := cfg.PGCfg.BuildConnectionURL()
	if err != nil {
		return "", errors.Wrap(err, "failed to build base connection URL")
	}

	poolConfig, err := cfg.Pool.BuildPoolConfig()
	if err != nil {
		return "", errors.Wrap(err, "failed to build pool config")
	}

	connURL = fmt.Sprintf("%s&%s", connURL, poolConfig)

	return connURL, nil
}
