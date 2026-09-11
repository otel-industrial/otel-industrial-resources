package ethernetipreceiver

import (
	"testing"
	"time"

	"github.com/danomagnum/gologix"
	"github.com/stretchr/testify/require"
)

// startTestServer starts a gologix CIP server on the fixed EtherNet/IP ports
// (44818/2222 — gologix.Server does not support a configurable bind address),
// seeded with the given tags.
//
// gologix.Server has no clean shutdown path: its accept loop retries forever
// on any Accept() error (including "closed network connection"), so Serve()
// never returns once started. We deliberately leak the server goroutines and
// listener for the life of the test binary rather than trying to synchronize
// on an exit that will never happen; see the goleak.IgnoreAnyFunction calls
// in generated_package_test.go (manually patched — see comment there).
//
// Because the ports are fixed, only one such server may run per test binary.
func startTestServer(t *testing.T, tagData map[string]any) {
	t.Helper()

	router := gologix.NewRouter()
	tags := &gologix.MapTagProvider{Data: tagData}

	path, err := gologix.ParsePath("1,0")
	require.NoError(t, err)
	router.Handle(path.Bytes(), tags)

	server := gologix.NewServer(router)

	go func() {
		_ = server.Serve()
	}()

	// Poll until the server accepts connections, rather than a fixed sleep.
	// Deliberately do not call Disconnect() on the probe: gologix.Client's
	// Disconnect can block for several seconds waiting on a response the
	// server side may never send once torn down, and this is just a
	// throwaway liveness probe.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		probe := gologix.NewClient("127.0.0.1")
		if err := probe.Connect(); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("test server did not become ready in time")
}

// TestClientAgainstRealServer covers the ReadTag type-fallback path and
// host/port handling against a single shared real gologix.Server instance
// (the server's fixed ports and lack of clean shutdown make running more
// than one instance per test binary unreliable).
func TestClientAgainstRealServer(t *testing.T) {
	startTestServer(t, map[string]any{
		"floattag": 3.14,
		"inttag":   int32(42),
		"booltag":  true,
	})

	t.Run("ReadTag type fallback", func(t *testing.T) {
		client := newGologixClient("127.0.0.1", 44818, 2*time.Second)
		require.NoError(t, client.Connect())

		tests := []struct {
			tag  string
			want float64
		}{
			{"floattag", 3.14},
			{"inttag", 42},
			{"booltag", 1},
		}

		for _, tt := range tests {
			t.Run(tt.tag, func(t *testing.T) {
				got, err := client.ReadTag(tt.tag)
				require.NoError(t, err)
				require.InDelta(t, tt.want, got, 0.0001)
			})
		}
	})

	t.Run("host formats", func(t *testing.T) {
		tests := []struct {
			name string
			host string
			port uint
		}{
			{"IP literal", "127.0.0.1", 44818},
			{"hostname", "localhost", 44818},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				client := newGologixClient(tt.host, tt.port, 2*time.Second)
				require.NoError(t, client.Connect())
			})
		}
	})

	t.Run("host must not include a port", func(t *testing.T) {
		// A host string with an embedded port is not a supported
		// configuration: Port is a separate field, and gologix treats
		// the combined string as the dial address, producing a
		// doubled/invalid port when Port is also set.
		client := newGologixClient("127.0.0.1:44818", 44818, 2*time.Second)
		require.Error(t, client.Connect())
	})
}
