package eapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"

	"ftp-worker/pkg/ctls"
	"ftp-worker/pkg/ctypes"
	"ftp-worker/pkg/keycloak"
)

const (
	biometricSamplePath = "/api/v1/in/sample"
	deletePath          = "/api/v1/delete"
)

type Client struct {
	host     *url.URL
	client   *http.Client
	keykloak *keycloak.Client
	Cfg      *Config
}

type Config struct {
	Host            string              `yaml:"host"`
	Timeout         ctypes.TimeDuration `yaml:"timeout"`
	Keycloak        keycloak.Config     `yaml:"keycloak"`
	Provider        string              `yaml:"provider"`
	TLS             ctls.Config         `yaml:"tls"`
	MaxIdleConns    int                 `yaml:"max_idle_conns"`
	MaxConns        int                 `yaml:"max_conns"`
	IdleConnTimeout ctypes.TimeDuration `yaml:"idle_conn_timeout"`
}

func New(cfg Config) (*Client, error) {
	host, err := url.Parse(cfg.Host)
	if err != nil {
		return nil, errors.Wrap(err, "api url parse")
	}

	keycloakClient, err := keycloak.New(cfg.Keycloak)
	if err != nil {
		return nil, errors.Wrap(err, "keycloak client")
	}

	tlsConfig, err := ctls.GetTLSConfig(cfg.TLS, host.Host)
	if err != nil {
		return nil, errors.Wrap(err, "get TLS config")
	}

	transport := &http.Transport{
		TLSClientConfig:     tlsConfig,
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConns,
		MaxConnsPerHost:     cfg.MaxConns,
		IdleConnTimeout:     cfg.IdleConnTimeout.Duration(),
	}

	return &Client{
		host:     host,
		client:   &http.Client{Transport: transport, Timeout: cfg.Timeout.Duration()},
		keykloak: keycloakClient,
		Cfg:      &cfg,
	}, nil
}

func (c *Client) BiometricSample(ctx context.Context, fieldName, fileName string, data []byte, params *Fields) (int, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		return http.StatusBadRequest, errors.Wrap(err, "failed to create from file")
	}

	_, err = io.Copy(part, bytes.NewReader(data))
	if err != nil {
		return http.StatusBadRequest, errors.Wrap(err, "failed to copy bytes")
	}

	byteParam, err := json.Marshal(params)
	if err != nil {
		return http.StatusBadRequest, errors.Wrap(err, "failed to marshal param")
	}

	err = writer.WriteField("params", string(byteParam))
	if err != nil {
		return http.StatusBadRequest, errors.Wrap(err, "failed to write field")
	}

	err = writer.Close()
	if err != nil {
		return http.StatusBadRequest, errors.Wrap(err, "failed to close writer")
	}

	c.host.Path = biometricSamplePath

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host.String(), body)
	if err != nil {
		return http.StatusBadRequest, errors.Wrap(err, "failed to create request")
	}

	jwtToken, err := c.keykloak.GetToken(ctx)
	if err != nil {
		return http.StatusBadRequest, errors.Wrap(err, "failed to get token")
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+jwtToken)

	response, err := c.client.Do(req)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "failed to send")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusAccepted {
		buf := bytes.Buffer{}
		if _, err = buf.ReadFrom(response.Body); err != nil {
			return response.StatusCode, fmt.Errorf("fail read response body from ebs-api service with status: %d", response.StatusCode)
		}

		return response.StatusCode, fmt.Errorf("error response from ebs-api service with status: %d and body: %s", response.StatusCode, buf.String())
	}

	return response.StatusCode, nil
}

func (c *Client) Del(ctx context.Context, params string) (int, error) {
	body := bytes.NewReader([]byte(params))

	c.host.Path = deletePath

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host.String(), body)
	if err != nil {
		return http.StatusBadRequest, errors.Wrap(err, "failed to create request")
	}

	jwtToken, err := c.keykloak.GetToken(ctx)
	if err != nil {
		return http.StatusBadRequest, errors.Wrap(err, "failed to get token")
	}

	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Authorization", "Bearer "+jwtToken)

	response, err := c.client.Do(req)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "failed to send")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		buf := bytes.Buffer{}
		if _, err = buf.ReadFrom(response.Body); err != nil {
			return response.StatusCode, fmt.Errorf("fail read response body from ebs-api service with status: %d", response.StatusCode)
		}

		return response.StatusCode, fmt.Errorf("error response from ebs-api service with status: %d and body: %s", response.StatusCode, buf.String())
	}

	return response.StatusCode, nil
}

func NewFields() *Fields {
	fields := &Fields{
		SubFields: &SubFields{},
		AgreeFields: &AgreeFields{
			&Agree{},
		},
		ExternalAuthPersonalFields: &ExternalAuthPersonalFields{
			ExternalAuthPerson: []ExternalAuthPerson{},
		},
		BiometricFields: &BiometricFields{},
	}

	return fields
}

type Fields struct {
	*SubFields
	*AgreeFields
	*ExternalAuthPersonalFields
	*BiometricFields
}

type SubFields struct {
	Sub string `json:"sub"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
}

type AgreeFields struct {
	Agree *Agree `json:"agree"`
}

type ExternalAuthPersonalFields struct {
	ExternalAuthPerson []ExternalAuthPerson `json:"matching"`
}

type BiometricFields struct {
	RegistrationSourceID        int                   `json:"registration_source_id"`
	SourceBiometricSampleID     int                   `json:"source_biometric_sample_id"`
	Aud                         []string              `json:"aud"`
	DatetimeLastBiometricSample time.Time             `json:"datetime_last_biometric_sample"`
	DatetimeTz                  int64                 `json:"datetime_tz"`
	BiometricCollecting         []BiometricCollecting `json:"bio_collecting"`
}

type Agree struct {
	AgreementID string     `json:"agreement_id"`
	DateFrom    time.Time  `json:"date_from"`
	DateTo      *time.Time `json:"date_to"`
}

type ExternalAuthPerson struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type BiometricCollecting struct {
	Name      string `json:"name"`
	Modality  string `json:"modality"`
	Signature string `json:"bio_sample_signature"`
}
