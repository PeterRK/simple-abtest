package sdk

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/peterrk/simple-abtest/engine/core"
)

// Client is an in-process AB decision client backed by periodic snapshots from
// engine service.
type Client struct {
	srcUrl  string
	client  *http.Client
	dataPtr *[]core.Experiment
	stamp   uint32
	active  bool
}

// NewClient creates an SDK client for one app.
// It performs an initial synchronous pull from engine `/app/:id`, then starts
// a background refresh loop. When ttl <= 1 minute, ttl is clamped to 1 minute.
func NewClient(address string, appid uint32, ttl time.Duration) (*Client, error) {
	c := new(Client)
	c.srcUrl = fmt.Sprintf("%s/app/%d", strings.TrimRight(address, "/"), appid)
	c.client = &http.Client{Timeout: time.Second * 10}

	if err := c.update(); err != nil {
		return nil, err
	}
	c.active = true

	if ttl <= time.Minute {
		ttl = time.Minute
	}
	go func() {
		for c.active {
			time.Sleep(ttl)
			c.update()
		}
	}()
	return c, nil
}

// Close stops the background refresh loop.
func (c *Client) Close() {
	c.active = false
}

func (c *Client) update() error {
	ptr := new([]core.Experiment)
	resp, err := c.client.Get(c.srcUrl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("fetch app info failed: status=%d", resp.StatusCode)
	}

	if err = json.NewDecoder(resp.Body).Decode(ptr); err != nil {
		return err
	}

	c.dataPtr = ptr
	c.stamp = uint32(time.Now().Unix())
	return nil
}

// Stamp returns the unix timestamp (seconds) of last successful update.
func (c *Client) Stamp() uint32 {
	return c.stamp
}

// AB evaluates experiments from the latest local snapshot for key/context.
// It returns nil values when no snapshot is available.
func (c *Client) AB(key string, ctx map[string]string) (cfg map[string]string, tags []string) {
	ptr := c.dataPtr
	if ptr == nil {
		return nil, nil
	}
	return core.GetExpConfig(*ptr, key, ctx)
}
