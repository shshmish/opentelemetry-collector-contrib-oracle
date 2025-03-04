// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package carbonreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.ReceiverConfig
	}{
		{
			id:       component.NewIDWithName(typeStr, ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(typeStr, "receiver_settings"),
			expected: &Config{
				ReceiverSettings: config.NewReceiverSettings(component.NewID(typeStr)),
				NetAddr: confignet.NetAddr{
					Endpoint:  "localhost:8080",
					Transport: "udp",
				},
				TCPIdleTimeout: 5 * time.Second,
				Parser: &protocol.Config{
					Type:   "plaintext",
					Config: &protocol.PlaintextConfig{},
				},
			},
		},
		{
			id: component.NewIDWithName(typeStr, "regex"),
			expected: &Config{
				ReceiverSettings: config.NewReceiverSettings(component.NewID(typeStr)),
				NetAddr: confignet.NetAddr{
					Endpoint:  "localhost:2003",
					Transport: "tcp",
				},
				TCPIdleTimeout: 30 * time.Second,
				Parser: &protocol.Config{
					Type: "regex",
					Config: &protocol.RegexParserConfig{
						Rules: []*protocol.RegexRule{
							{
								Regexp:     `(?P<key_base>test)\.env(?P<key_env>[^.]*)\.(?P<key_host>[^.]*)`,
								NamePrefix: "name-prefix",
								Labels: map[string]string{
									"dot.key": "dot.value",
									"key":     "value",
								},
								MetricType: "cumulative",
							},
							{
								Regexp: `(?P<key_just>test)\.(?P<key_match>.*)`,
							},
						},
						MetricNameSeparator: "_",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalReceiverConfig(sub, cfg))

			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
