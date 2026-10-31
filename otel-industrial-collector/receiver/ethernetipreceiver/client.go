package ethernetipreceiver

import (
	"fmt"
	"strconv"
	"strings"
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

func newGologixClient(host string, port uint, timeout time.Duration) *gologixClient {
	client := gologix.NewClient(host)
	if port > 0 {
		client.Controller.Port = port
	}
	if timeout > 0 {
		client.SocketTimeout = timeout
	}
	return &gologixClient{
		endpoint: host,
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
	// Unlike Read, gologix's ListIdentity writes to the socket without
	// checking that one exists, so it panics after a failed Connect (#54).
	if !c.client.Connected() {
		c.connected = false
		return fmt.Errorf("pinging %s: not connected", c.endpoint)
	}
	if _, err := c.client.ListIdentity(); err != nil {
		c.connected = false
		return fmt.Errorf("pinging %s: %w", c.endpoint, err)
	}
	c.connected = true
	return nil
}

// ReadTag reads a single tag with one request and returns its value as a
// float64. gologix decodes the value by the CIP type the controller reports,
// so any atomic type can be read without knowing it in advance.
//
// A bit tag such as Status.3 is the exception. gologix drops the bit number
// from the request and only extracts that bit from the returned word when
// asked for a BOOL; otherwise it returns the whole word.
func (c *gologixClient) ReadTag(tagName string) (float64, error) {
	typ := gologix.CIPTypeUnknown
	if isBitTag(tagName) {
		typ = gologix.CIPTypeBOOL
	}
	v, err := c.client.Read_single(tagName, typ, 1)
	if err != nil {
		return 0, fmt.Errorf("reading tag %q: %w", tagName, err)
	}
	f, ok := toFloat64(v)
	if !ok {
		return 0, fmt.Errorf("reading tag %q: unsupported type %T, only atomic types can be read", tagName, v)
	}
	return f, nil
}

// isBitTag reports whether gologix will treat tagName as a bit access: a
// final dotted part that is a bit number from 0 to 31.
func isBitTag(tagName string) bool {
	i := strings.LastIndexByte(tagName, '.')
	if i < 0 {
		return false
	}
	n, err := strconv.Atoi(tagName[i+1:])
	return err == nil && n >= 0 && n <= 31
}

// toFloat64 converts a decoded atomic CIP value. Structs, strings and arrays
// arrive as other Go types and are rejected.
func toFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	case int8:
		return float64(x), true
	case uint8:
		return float64(x), true
	case int16:
		return float64(x), true
	case uint16:
		return float64(x), true
	case int32:
		return float64(x), true
	case uint32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint64:
		return float64(x), true
	case float32:
		return float64(x), true
	case float64:
		return x, true
	}
	return 0, false
}
