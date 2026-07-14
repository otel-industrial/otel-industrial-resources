package ethernetipreceiver

import (
	"fmt"

	"github.com/danomagnum/gologix"
)

// cipClient is a narrow interface over the parts of gologix we use.
// Keeping this separate from gologix.Client lets us fake it in tests
// without a live PLC connection.
type cipClient interface {
	Connect() error
	Disconnect() error
	IsConnected() bool
	ReadTag(tagName string) (float64, error)
}

// gologixClient adapts *gologix.Client to the cipClient interface.
type gologixClient struct {
	endpoint  string
	client    *gologix.Client
	connected bool
}

func newGologixClient(endpoint string) *gologixClient {
	return &gologixClient{
		endpoint: endpoint,
		client:   gologix.NewClient(endpoint),
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

func (c *gologixClient) IsConnected() bool {
	return c.connected
}

// ReadTag reads a single tag and returns its value as a float64.
// This is a simplification for the initial implementation: gologix.Read
// requires a typed destination, so we try float64 directly. Tags with
// other underlying types (bool, int, etc.) will need broader type
// handling in a later milestone.
func (c *gologixClient) ReadTag(tagName string) (float64, error) {
	var value float64
	if err := c.client.Read(tagName, &value); err != nil {
		return 0, fmt.Errorf("reading tag %q: %w", tagName, err)
	}
	return value, nil
}
