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
}

// gologixClient adapts *gologix.Client to the cipClient interface.
type gologixClient struct {
	endpoint string
	client   *gologix.Client
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
