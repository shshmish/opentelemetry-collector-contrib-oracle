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

package resourceprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.NotNil(t, cfg)
}

func TestCreateProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{
		ProcessorSettings: config.NewProcessorSettings(component.NewID(typeStr)),
		AttributesActions: []attraction.ActionKeyValue{
			{Key: "cloud.availability_zone", Value: "zone-1", Action: attraction.UPSERT},
		},
	}

	tp, err := factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tp)

	mp, err := factory.CreateMetricsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, mp)
}

func TestInvalidEmptyActions(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	_, err := factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err)

	_, err = factory.CreateMetricsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err)
}

func TestInvalidAttributeActions(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{
		ProcessorSettings: config.NewProcessorSettings(component.NewID(typeStr)),
		AttributesActions: []attraction.ActionKeyValue{
			{Key: "k", Value: "v", Action: "invalid-action"},
		},
	}

	_, err := factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, nil)
	assert.Error(t, err)

	_, err = factory.CreateMetricsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, nil)
	assert.Error(t, err)
}
