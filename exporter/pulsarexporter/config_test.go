// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pulsarexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.ExporterConfig
	}{
		{
			id: component.NewIDWithName(typeStr, ""),
			expected: &Config{
				ExporterSettings: config.NewExporterSettings(component.NewID(typeStr)),
				TimeoutSettings: exporterhelper.TimeoutSettings{
					Timeout: 20 * time.Second,
				},
				RetrySettings: exporterhelper.RetrySettings{
					Enabled:         true,
					InitialInterval: 10 * time.Second,
					MaxInterval:     1 * time.Minute,
					MaxElapsedTime:  10 * time.Minute,
				},
				QueueSettings: exporterhelper.QueueSettings{
					Enabled:      true,
					NumConsumers: 2,
					QueueSize:    10,
				},
				Endpoint:              "pulsar://localhost:6650",
				Topic:                 "spans",
				Encoding:              "otlp-spans",
				TLSTrustCertsFilePath: "ca.pem",
				Authentication:        Authentication{TLS: &TLS{CertFile: "cert.pem", KeyFile: "key.pem"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalExporterConfig(sub, cfg))

			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestClientOptions(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(typeStr, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalExporterConfig(sub, cfg))

	options := cfg.(*Config).clientOptions()

	assert.Equal(t, &pulsar.ClientOptions{
		URL:                   "pulsar://localhost:6650",
		TLSTrustCertsFilePath: "ca.pem",
		Authentication:        pulsar.NewAuthenticationTLS("cert.pem", "key.pem"),
	}, &options)

}
