// Copyright 2020, OpenTelemetry Authors
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

package kubeletstatsreceiver

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	kube "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	duration := 10 * time.Second

	tests := []struct {
		id          component.ID
		expected    component.ReceiverConfig
		expectedErr error
	}{
		{
			id: component.NewIDWithName(typeStr, "default"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					ReceiverSettings:   config.NewReceiverSettings(component.NewID(typeStr)),
					CollectionInterval: duration,
				},
				ClientConfig: kube.ClientConfig{
					APIConfig: k8sconfig.APIConfig{
						AuthType: "tls",
					},
				},
				MetricGroupsToCollect: []kubelet.MetricGroup{
					kubelet.ContainerMetricGroup,
					kubelet.PodMetricGroup,
					kubelet.NodeMetricGroup,
				},
				Metrics: metadata.DefaultMetricsSettings(),
			},
		},
		{
			id: component.NewIDWithName(typeStr, "tls"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					ReceiverSettings:   config.NewReceiverSettings(component.NewID(typeStr)),
					CollectionInterval: duration,
				},
				TCPAddr: confignet.TCPAddr{
					Endpoint: "1.2.3.4:5555",
				},
				ClientConfig: kube.ClientConfig{
					APIConfig: k8sconfig.APIConfig{
						AuthType: "tls",
					},
					TLSSetting: configtls.TLSSetting{
						CAFile:   "/path/to/ca.crt",
						CertFile: "/path/to/apiserver.crt",
						KeyFile:  "/path/to/apiserver.key",
					},
					InsecureSkipVerify: true,
				},
				MetricGroupsToCollect: []kubelet.MetricGroup{
					kubelet.ContainerMetricGroup,
					kubelet.PodMetricGroup,
					kubelet.NodeMetricGroup,
				},
				Metrics: metadata.DefaultMetricsSettings(),
			},
		},
		{
			id: component.NewIDWithName(typeStr, "sa"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					ReceiverSettings:   config.NewReceiverSettings(component.NewID(typeStr)),
					CollectionInterval: duration,
				},
				ClientConfig: kube.ClientConfig{
					APIConfig: k8sconfig.APIConfig{
						AuthType: "serviceAccount",
					},
					InsecureSkipVerify: true,
				},
				MetricGroupsToCollect: []kubelet.MetricGroup{
					kubelet.ContainerMetricGroup,
					kubelet.PodMetricGroup,
					kubelet.NodeMetricGroup,
				},
				Metrics: metadata.DefaultMetricsSettings(),
			},
		},
		{
			id: component.NewIDWithName(typeStr, "metadata"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					ReceiverSettings:   config.NewReceiverSettings(component.NewID(typeStr)),
					CollectionInterval: duration,
				},
				ClientConfig: kube.ClientConfig{
					APIConfig: k8sconfig.APIConfig{
						AuthType: "serviceAccount",
					},
				},
				ExtraMetadataLabels: []kubelet.MetadataLabel{
					kubelet.MetadataLabelContainerID,
					kubelet.MetadataLabelVolumeType,
				},
				MetricGroupsToCollect: []kubelet.MetricGroup{
					kubelet.ContainerMetricGroup,
					kubelet.PodMetricGroup,
					kubelet.NodeMetricGroup,
				},
				Metrics: metadata.DefaultMetricsSettings(),
			},
		},
		{
			id: component.NewIDWithName(typeStr, "metric_groups"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					ReceiverSettings:   config.NewReceiverSettings(component.NewID(typeStr)),
					CollectionInterval: 20 * time.Second,
				},
				ClientConfig: kube.ClientConfig{
					APIConfig: k8sconfig.APIConfig{
						AuthType: "serviceAccount",
					},
				},
				MetricGroupsToCollect: []kubelet.MetricGroup{
					kubelet.PodMetricGroup,
					kubelet.NodeMetricGroup,
					kubelet.VolumeMetricGroup,
				},
				Metrics: metadata.DefaultMetricsSettings(),
			},
		},
		{
			id: component.NewIDWithName(typeStr, "metadata_with_k8s_api"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					ReceiverSettings:   config.NewReceiverSettings(component.NewID(typeStr)),
					CollectionInterval: duration,
				},
				ClientConfig: kube.ClientConfig{
					APIConfig: k8sconfig.APIConfig{
						AuthType: "serviceAccount",
					},
				},
				ExtraMetadataLabels: []kubelet.MetadataLabel{
					kubelet.MetadataLabelVolumeType,
				},
				MetricGroupsToCollect: []kubelet.MetricGroup{
					kubelet.ContainerMetricGroup,
					kubelet.PodMetricGroup,
					kubelet.NodeMetricGroup,
				},
				K8sAPIConfig: &k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeKubeConfig},
				Metrics:      metadata.DefaultMetricsSettings(),
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

func TestGetReceiverOptions(t *testing.T) {
	type fields struct {
		extraMetadataLabels   []kubelet.MetadataLabel
		metricGroupsToCollect []kubelet.MetricGroup
		k8sAPIConfig          *k8sconfig.APIConfig
	}
	tests := []struct {
		name    string
		fields  fields
		want    *scraperOptions
		wantErr bool
	}{
		{
			name: "Valid config",
			fields: fields{
				extraMetadataLabels: []kubelet.MetadataLabel{
					kubelet.MetadataLabelContainerID,
				},
				metricGroupsToCollect: []kubelet.MetricGroup{
					kubelet.NodeMetricGroup,
					kubelet.PodMetricGroup,
				},
			},
			want: &scraperOptions{
				id: component.NewID(typeStr),
				extraMetadataLabels: []kubelet.MetadataLabel{
					kubelet.MetadataLabelContainerID,
				},
				metricGroupsToCollect: map[kubelet.MetricGroup]bool{
					kubelet.NodeMetricGroup: true,
					kubelet.PodMetricGroup:  true,
				},
				collectionInterval: 10 * time.Second,
			},
		},
		{
			name: "Invalid metric group",
			fields: fields{
				extraMetadataLabels: []kubelet.MetadataLabel{
					"unsupported",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Invalid extra metadata",
			fields: fields{
				metricGroupsToCollect: []kubelet.MetricGroup{
					"unsupported",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Fails to create k8s API client",
			fields: fields{
				k8sAPIConfig: &k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					ReceiverSettings:   config.NewReceiverSettings(component.NewID(typeStr)),
					CollectionInterval: 10 * time.Second,
				},
				ExtraMetadataLabels:   tt.fields.extraMetadataLabels,
				MetricGroupsToCollect: tt.fields.metricGroupsToCollect,
				K8sAPIConfig:          tt.fields.k8sAPIConfig,
			}
			got, err := cfg.getReceiverOptions()
			if (err != nil) != tt.wantErr {
				t.Errorf("getReceiverOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getReceiverOptions() got = %v, want %v", got, tt.want)
			}
		})
	}
}
