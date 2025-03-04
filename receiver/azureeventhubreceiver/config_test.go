// Copyright The OpenTelemetry Authors
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

package azureeventhubreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 2)

	r0 := cfg.Receivers[component.NewID(typeStr)]
	assert.Equal(t, "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName", r0.(*Config).Connection)
	assert.Equal(t, "", r0.(*Config).Offset)
	assert.Equal(t, "", r0.(*Config).Partition)

	r1 := cfg.Receivers[component.NewIDWithName(typeStr, "all")]
	assert.Equal(t, "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName", r1.(*Config).Connection)
	assert.Equal(t, "1234-5566", r1.(*Config).Offset)
	assert.Equal(t, "foo", r1.(*Config).Partition)
}

func TestMissingConnection(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	err := cfg.Validate()
	assert.EqualError(t, err, "missing connection")
}

func TestInvalidConnectionString(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).Connection = "foo"
	err := cfg.Validate()
	assert.EqualError(t, err, "failed parsing connection string due to unmatched key value separated by '='")
}
