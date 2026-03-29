package sdk

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/peterrk/simple-abtest/engine/core"
	"github.com/peterrk/simple-abtest/utils"
)

type snapshot struct {
	exps  []core.Experiment
	stamp uint32
}

// Client is an in-process AB decision client backed by periodic snapshots from
// engine service.
type Client struct {
	srcUrl string
	token  string
	client *http.Client

	ctx    context.Context
	cancel context.CancelFunc
	data   atomic.Pointer[snapshot]
}

// NewClient creates an SDK client for one app.
// It performs an initial synchronous pull from engine `/app/:id`, then starts
// a background refresh loop. When ttl <= 1 minute, ttl is clamped to 1 minute.
func NewClient(address string, appid uint32, accessToken string,
	ttl time.Duration) (*Client, error) {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Client{
		srcUrl: fmt.Sprintf("%s/app/%d", strings.TrimRight(address, "/"), appid),
		token:  accessToken,
		client: newHTTPClient(10 * time.Second),
		ctx:    ctx,
		cancel: cancel,
	}

	if err := c.update(); err != nil {
		cancel()
		return nil, err
	}

	if ttl == 0 {
		return c, nil
	}
	if ttl <= time.Minute {
		ttl = time.Minute
	}
	go c.refreshLoop(ttl)
	return c, nil
}

// Close stops the background refresh loop.
func (c *Client) Close() {
	if c.cancel != nil {
		c.cancel()
	}
}

func (c *Client) update() error {
	exps := make([]core.Experiment, 0)
	if _, err := c.restfulGet(c.ctx, c.srcUrl, map[string]string{
		"ACCESS_TOKEN": c.token,
	}, nil, &exps); err != nil {
		return err
	}

	c.data.Store(&snapshot{
		exps:  exps,
		stamp: uint32(time.Now().Unix()),
	})
	return nil
}

// Stamp returns the unix timestamp (seconds) of last successful update.
func (c *Client) Stamp() uint32 {
	ptr := c.data.Load()
	if ptr == nil {
		return 0
	}
	return ptr.stamp
}

// AB evaluates experiments from the latest local snapshot for key/context.
// It returns nil values when no snapshot is available.
func (c *Client) AB(key string, ctx map[string]string) (cfg map[string]string, tags []string) {
	ptr := c.data.Load()
	if ptr == nil {
		return nil, nil
	}
	return core.GetExpConfig(ptr.exps, key, ctx)
}

func (c *Client) refreshLoop(ttl time.Duration) {
	for c.ctx.Err() == nil {
		time.Sleep(ttl)
		if c.ctx.Err() != nil {
			return
		}
		_ = c.update()
	}
}

func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			MaxIdleConns:          32,
			MaxIdleConnsPerHost:   16,
			MaxConnsPerHost:       16,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Timeout: timeout,
	}
}

func (c *Client) restfulGet(ctx context.Context, url string,
	headers, params map[string]string, out any) (int, error) {
	req, err := utils.NewRestfulRequest(ctx, http.MethodGet, url, headers, params, nil)
	if err != nil {
		return 0, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	return utils.HandleRestfulResponse(resp, out)
}
