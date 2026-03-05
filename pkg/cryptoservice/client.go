package cryptoservice

import (
	"context"
	"net"
	"net/url"

	"github.com/pkg/errors"
	"golang.org/x/net/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"ftp-worker/pkg/ctls"
	pb "ftp-worker/proto/crypto/v1"
)

const maxMessageSize = 10 * 1024 * 1024

type Client struct {
	conn pb.CryptoServiceClient
	Cfg  *Config
}

func New(cfg Config) (*Client, error) {
	if cfg.MaxMessageSize <= 0 {
		cfg.MaxMessageSize = maxMessageSize
	}

	tlsConfig, err := ctls.GetTLSConfig(cfg.TLS, cfg.Address)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get TLS config")
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(cfg.MaxMessageSize)),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(cfg.MaxMessageSize)),
	}

	if len(cfg.ProxyURI) > 0 {
		proxyURL, err := url.Parse(cfg.ProxyURI)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse proxy URI")
		}

		dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
		if err != nil {
			panic(err)
		}

		opts = append(opts, grpc.WithContextDialer(func(ctx context.Context, uri string) (net.Conn, error) {
			return dialer.Dial("tcp", uri)
		}))
	}

	conn, err := grpc.NewClient(cfg.Address, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create gRPC client")
	}

	return &Client{
		conn: pb.NewCryptoServiceClient(conn),
		Cfg:  &cfg,
	}, nil
}

func (c *Client) SignDetachedDataWithCert(ctx context.Context, signData []byte, alg string, sType string, cn string) ([]byte, error) {
	request := &pb.SignDataRequest{
		Indexs: [][]byte{signData},
		Parameters: &pb.Parameters{
			Algorithm: alg,
			Type:      sType,
			KeyId:     cn,
		},
	}

	response, err := c.conn.SignDetachedDataWithCert(ctx, request)
	if err == nil && len(response.Error) != 0 {
		err = errors.New(response.Error)
	}

	if err != nil {
		return nil, errors.Wrap(err, "failed to sign data")
	}

	if len(response.Signatures) > 0 {
		if response.Signatures[0].Error == 0 {
			return response.Signatures[0].Value, nil
		}
	}

	return nil, errors.New("failed to sign data")
}

func (c *Client) CreateJWTWithCert(ctx context.Context, payload, alg, sType, cn string) (string, error) {
	request := &pb.CreateJWTRequest{
		Payload: payload,
		Parameters: &pb.Parameters{
			Algorithm: alg,
			Type:      sType,
			KeyId:     cn,
		},
	}

	response, err := c.conn.CreateJWTCert(ctx, request)
	if err == nil && len(response.Error) != 0 {
		err = errors.New(response.Error)
	}

	if err != nil {
		return "", errors.Wrap(err, "failed to create JWT")
	}

	if response.Error != "" {
		return "", errors.New("CreateJWTWithCert failed with error: " + response.Error)
	}

	return response.Jwt, nil
}

type Config struct {
	Address        string      `yaml:"address"`
	ProxyURI       string      `yaml:"proxy_uri"`
	MaxMessageSize int         `yaml:"max_message_size"`
	Certificate    string      `yaml:"certificate"`
	TLS            ctls.Config `yaml:"tls"`
}
