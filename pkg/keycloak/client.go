package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/pkg/errors"

	"ftp-worker/pkg/ctls"
	"ftp-worker/pkg/ctypes"
)

type Client struct {
	url      url.URL
	provider http.Client
	store    store
	cfg      Config
}

type Config struct {
	Host         string              `yaml:"host"`
	Timeout      ctypes.TimeDuration `yaml:"timeout"`
	Realm        string              `yaml:"realm"`
	ClientID     string              `yaml:"client_id"`
	ClientSecret string              `yaml:"client_secret"`
	Username     string              `yaml:"username"`
	Password     string              `yaml:"password"`
	TLS          ctls.Config         `yaml:"tls"`
}

func New(cfg Config) (*Client, error) {
	host, err := url.Parse(cfg.Host)
	if err != nil {
		return nil, errors.Wrap(err, "keyckoak provider")
	}

	tlsConfig, err := ctls.GetTLSConfig(cfg.TLS, host.Host)
	if err != nil {
		return nil, errors.Wrap(err, "get TLS config")
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return &Client{
		url:      *host,
		provider: http.Client{Transport: transport, Timeout: cfg.Timeout.Duration()},
		store:    store{},
		cfg:      cfg,
	}, nil
}

func (c *Client) GetToken(ctx context.Context) (string, error) {
	if c.store.IsEmpty() {
		res, err := c.getToken(ctx, false)
		if err != nil {
			return "", errors.Wrap(err, "get token")
		}

		c.store.Set(res, false)

		return res.AccessToken, nil
	}

	if c.store.TokenIsValid() {
		return c.store.GetToken(), nil
	}

	if c.store.RefreshTokenIsValid() {
		res, err := c.getToken(ctx, true)
		if err != nil {
			return "", errors.Wrap(err, "refresh token")
		}

		c.store.Set(res, true)

		return res.AccessToken, nil
	}

	res, err := c.getToken(ctx, false)
	if err != nil {
		return "", errors.Wrap(err, "get token")
	}

	c.store.Set(res, false)

	return res.AccessToken, nil
}

func (c *Client) getToken(ctx context.Context, isRefresh bool) (*resp, error) {
	data := url.Values{}

	data.Set("client_id", c.cfg.ClientID)
	data.Set("client_secret", c.cfg.ClientSecret)

	if isRefresh {
		data.Set("grant_type", "refresh_token")
		data.Set("refresh_token", c.store.GetRefreshToken())
	} else {
		data.Set("grant_type", "password")
		data.Set("username", c.cfg.Username)
		data.Set("password", c.cfg.Password)
	}

	path := strings.Builder{}

	path.WriteString("/realms/")
	path.WriteString(c.cfg.Realm)
	path.WriteString("/protocol/openid-connect/token")

	c.url.Path = path.String()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url.String(), strings.NewReader(data.Encode()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := c.provider.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		buf := bytes.Buffer{}

		if _, err = buf.ReadFrom(response.Body); err != nil {
			return nil, fmt.Errorf("fail read response body from keycloak service with status: %d", response.StatusCode)
		}

		return nil, fmt.Errorf("error response from keycloak service with status: %d and body: %s", response.StatusCode, buf.String())
	}

	res := new(resp)

	if err = json.NewDecoder(response.Body).Decode(res); err != nil {
		return nil, errors.Wrap(err, "decode response body")
	}

	return res, nil
}

type resp struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	NotBeforePolicy  int    `json:"not-before-policy"`
	SessionState     string `json:"session_state"`
	Scope            string `json:"scope"`
}
