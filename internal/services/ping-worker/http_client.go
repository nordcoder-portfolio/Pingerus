package ping_worker

import (
	"context"
	"crypto/tls"
	config "github.com/NordCoder/Pingerus/internal/config/ping-worker"
	"net"
	"net/http"
	"time"
)

type Config struct {
	Timeout         time.Duration
	UserAgent       string
	FollowRedirects bool
	VerifyTLS       bool
}

type Client struct {
	c   *http.Client
	cfg config.HTTPPing
}

func New(cfg config.HTTPPing) *Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   cfg.Timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !cfg.VerifyTLS,
			MinVersion:         tls.VersionTLS12,
		},
	}
	client := &http.Client{Timeout: cfg.Timeout, Transport: transport}
	if !cfg.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return &Client{c: client, cfg: cfg}
}

func (cl *Client) Ping(ctx context.Context, url, userAgent string) (int, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, false, err
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	resp, err := cl.c.Do(req)
	if err != nil {
		return 0, false, nil
	}
	defer resp.Body.Close()
	code := resp.StatusCode
	up := code >= 200 && code <= 399
	return code, up, nil
}
