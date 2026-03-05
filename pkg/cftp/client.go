package cftp

import (
	"context"
	"crypto/tls"
	"net/http"
	"strconv"

	"github.com/jlaffaye/ftp"
	"github.com/pkg/errors"

	"ftp-worker/pkg/ctls"
	"ftp-worker/pkg/ctypes"
)

type Config struct {
	Address      string              `yaml:"address"`
	Port         int                 `yaml:"port"`
	Login        string              `yaml:"login"`
	Password     string              `yaml:"password"`
	Timeout      ctypes.TimeDuration `yaml:"timeout"`
	Root         string              `yaml:"root"`
	OutputFolder string              `yaml:"output_folder"`
	DisableUTF8  bool                `yaml:"disable_utf8"`
	TLSType      string              `yaml:"tls_type"` // no|implict|explict
	TLSConfig    ctls.Config         `yaml:"tls_config"`
}

func CreateConn(ctx context.Context, conf *Config) (*ftp.ServerConn, int, error) {
	ftpAddress := conf.Address

	if conf.Port > 0 {
		ftpAddress = conf.Address + ":" + strconv.Itoa(conf.Port)
	}

	options, err := buildFTPOptions(ctx, conf)
	if err != nil {
		return nil, 0, errors.Wrap(err, "ftp build options")
	}

	conn, err := ftp.Dial(ftpAddress, options...)
	if err != nil {
		return nil, http.StatusServiceUnavailable, errors.Wrap(err, "ftp dial")
	}

	err = conn.Login(conf.Login, conf.Password)
	if err != nil {
		return nil, http.StatusForbidden, errors.Wrap(err, "ftp login")
	}

	err = conn.Type(ftp.TransferTypeBinary)
	if err != nil {
		return nil, http.StatusInternalServerError, errors.Wrap(err, "ftp set transfer type")
	}

	return conn, 0, nil
}

func buildFTPOptions(ctx context.Context, cfg *Config) ([]ftp.DialOption, error) {
	opts := ftp.DialWithContext(ctx)

	res := []ftp.DialOption{opts}

	if cfg.Timeout > 0 {
		res = append(res, ftp.DialWithTimeout(cfg.Timeout.Duration()))
	}

	res = append(res, ftp.DialWithDisabledUTF8(cfg.DisableUTF8))

	var (
		tlsConfig *tls.Config
		err       error
	)

	if cfg.TLSType != "" && cfg.TLSType != "no" {
		tlsConfig, err = ctls.GetTLSConfig(cfg.TLSConfig, cfg.Address)
		if err != nil {
			return res, err
		}
	}

	switch cfg.TLSType {
	case "implict":
		res = append(res, ftp.DialWithTLS(tlsConfig))
	case "explict":
		res = append(res, ftp.DialWithExplicitTLS(tlsConfig))
	default:
	}

	return res, nil
}
