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

package splunkhecreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:1" // Endpoint is required, not going to be used here.

	mockLogsConsumer := consumertest.NewNop()
	lReceiver, err := createLogsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, mockLogsConsumer)
	assert.Nil(t, err, "receiver creation failed")
	assert.NotNil(t, lReceiver, "receiver creation failed")

	mockMetricsConsumer := consumertest.NewNop()
	mReceiver, err := createMetricsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, mockMetricsConsumer)
	assert.Nil(t, err, "receiver creation failed")
	assert.NotNil(t, mReceiver, "receiver creation failed")
}

func TestFactoryType(t *testing.T) {
	assert.Equal(t, component.Type("splunk_hec"), NewFactory().Type())
}

func TestCreateNilNextConsumerMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:1"

	mReceiver, err := createMetricsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, nil)
	assert.EqualError(t, err, "nil metricsConsumer")
	assert.Nil(t, mReceiver, "receiver creation failed")
}

func TestCreateNilNextConsumerLogs(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:1"

	mReceiver, err := createLogsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, nil)
	assert.EqualError(t, err, "nil logsConsumer")
	assert.Nil(t, mReceiver, "receiver creation failed")
}
