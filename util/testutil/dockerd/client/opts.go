package client

import (
	"net/http"

	"github.com/docker/go-connections/sockets"
	"github.com/pkg/errors"
)

type Opt func(*Client) error

func WithHost(host string) Opt {
	return func(c *Client) error {
		hostURL, err := parseHostURL(host)
		if err != nil {
			return err
		}
		c.host = host
		c.proto = hostURL.Scheme
		c.addr = hostURL.Host
		c.basePath = hostURL.Path
		if transport, ok := c.client.Transport.(*http.Transport); ok {
			return sockets.ConfigureTransport(transport, c.proto, c.addr)
		}
		return errors.Errorf("cannot apply host to transport: %T", c.client.Transport)
	}
}
