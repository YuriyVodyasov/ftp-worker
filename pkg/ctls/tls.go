package ctls

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"

	"github.com/pkg/errors"
)

type Config struct {
	ClientCert         string   `yaml:"client_cert"`
	ClientKey          string   `yaml:"client_key"`
	CACert             string   `yaml:"ca_cert"`
	ServerName         string   `yaml:"server_name"`
	Scenario           Scenario `yaml:"scenario"` // "domain", "ip"
	InsecureSkipVerify bool     `yaml:"insecure_skip_verify"`
}

func GetTLSConfig(cfg Config, dialAddr string) (*tls.Config, error) {
	var certificates []tls.Certificate

	if cfg.ClientCert != "" && cfg.ClientKey != "" {
		clientCert, err := tls.LoadX509KeyPair(cfg.ClientCert, cfg.ClientKey)
		if err != nil {
			return nil, errors.Wrap(err, "load client certificate")
		}

		certificates = []tls.Certificate{clientCert}
	}

	caCertPool := x509.NewCertPool()

	if cfg.CACert != "" {
		caCert, err := os.ReadFile(cfg.CACert)
		if err != nil {
			return nil, errors.Wrap(err, "read CA certificate")
		}

		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, errors.New("failed to append CA certificate") // should we write this error to log instead of returning it?
		}
	}

	switch cfg.Scenario {
	case ScenarioDomain:
		if cfg.ServerName == "" {
			host, _, err := net.SplitHostPort(dialAddr)
			if err != nil {
				host = dialAddr
			}

			cfg.ServerName = host
		}

	case ScenarioIP:
		if dialAddr == "" || cfg.ServerName == "" {
			return nil, errors.New("invalid config for IP scenario: missing dial address or server name")
		}
	}

	tlsConfig := &tls.Config{
		Certificates:       certificates,
		RootCAs:            caCertPool,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		MinVersion:         tls.VersionTLS13, // enforce TLS 1.3 for better security and to avoid issues with older versions
		ServerName:         cfg.ServerName,
	}

	return tlsConfig, nil
}
