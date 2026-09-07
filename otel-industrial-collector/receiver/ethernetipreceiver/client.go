package ethernetipreceiver

import (
	"fmt"
	"time"

	"github.com/danomagnum/gologix"
)

// cipClient is a narrow interface over the parts of gologix we use.
// Keeping this separate from gologix.Client lets us fake it in tests
// without a live PLC connection.
type cipClient interface {
	Connect() error
	Disconnect() error
	// Ping performs a lightweight round-trip to the device (EtherNet/IP
	// ListIdentity) to actively verify the connection is still alive,
	// rather than trusting a locally cached connected/disconnected flag.
	Ping() error
	ReadTag(tagName string) (float64, error)
}

// gologixClient adapts *gologix.Client to the cipClient interface.
type gologixClient struct {
	endpoint  string
	client    *gologix.Client
	connected bool
}

func newGologixClient(endpoint string, timeout time.Duration) *gologixClient {
	client := gologix.NewClient(endpoint)
	if timeout > 0 {
		client.SocketTimeout = timeout
	}
	return &gologixClient{
		endpoint: endpoint,
		client:   client,
	}
}

func (c *gologixClient) Connect() error {
	if err := c.client.Connect(); err != nil {
		c.connected = false
		return fmt.Errorf("connecting to %s: %w", c.endpoint, err)
	}
	c.connected = true
	return nil
}

func (c *gologixClient) Disconnect() error {
	err := c.client.Disconnect()
	c.connected = false
	return err
}

// Ping actively checks the connection by issuing a ListIdentity request.
// A cached "connected" flag can go stale if the socket drops between
// scrapes, so this performs a real round-trip on every call instead.
func (c *gologixClient) Ping() error {
	if _, err := c.client.ListIdentity(); err != nil {
		c.connected = false
		return fmt.Errorf("pinging %s: %w", c.endpoint, err)
	}
	c.connected = true
	return nil
}

// ReadTag reads a single tag and returns its value as a float64.
//
// This is a simplification for the initial implementation: gologix.Read
// requires a destination matching the tag's underlying CIP type, so we
// try the common numeric types in order until one succeeds. A later
// milestone should read the tag's actual type via ListAllTags/discovery
// instead of guessing.
func (c *gologixClient) ReadTag(tagName string) (float64, error) {
	var f64 float64
	if err := c.client.Read(tagName, &f64); err == nil {
		return f64, nil
	}

	var i32 int32
	if err := c.client.Read(tagName, &i32); err == nil {
		return float64(i32), nil
	}

	var b bool
	if err := c.client.Read(tagName, &b); err == nil {
		if b {
			return 1, nil
		}
		return 0, nil
	}

	return 0, fmt.Errorf("reading tag %q: unsupported or unreadable type", tagName)
}
