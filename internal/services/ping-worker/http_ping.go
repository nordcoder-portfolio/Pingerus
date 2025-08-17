package ping_worker

import (
	"context"
)

type HTTPPinger interface {
	Ping(ctx context.Context, url, userAgent string) (code int, up bool, err error)
}

type HTTPPingCfg struct {
	UserAgent string
}

type HTTPPing struct {
	Client    HTTPPinger
	UserAgent string
}

func (h HTTPPing) Do(ctx context.Context, url string) (code int, up bool, err error) {
	return h.Client.Ping(ctx, url, h.UserAgent)
}
