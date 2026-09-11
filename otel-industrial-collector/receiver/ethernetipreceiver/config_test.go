package ethernetipreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name:    "missing host",
			cfg:     Config{Host: "", Tags: []string{"TagA"}},
			wantErr: "host must be specified",
		},
		{
			name:    "missing tags",
			cfg:     Config{Host: "127.0.0.1", Tags: nil},
			wantErr: "at least one tag must be specified",
		},
		{
			name: "valid config",
			cfg:  Config{Host: "127.0.0.1", Tags: []string{"TagA"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
