// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deltatorateprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatorateprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "deltatorate"
	// The stability level of the processor.
	stability = component.StabilityLevelInDevelopment
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory returns a new factory for the Delta to Rate processor.
func NewFactory() component.ProcessorFactory {
	return component.NewProcessorFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsProcessor(createMetricsProcessor, stability))
}

func createDefaultConfig() component.ProcessorConfig {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(component.NewID(typeStr)),
	}
}

func createMetricsProcessor(
	ctx context.Context,
	set component.ProcessorCreateSettings,
	cfg component.ProcessorConfig,
	nextConsumer consumer.Metrics,
) (component.MetricsProcessor, error) {
	processorConfig, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("configuration parsing error")
	}

	metricsProcessor := newDeltaToRateProcessor(processorConfig, set.Logger)

	return processorhelper.NewMetricsProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		metricsProcessor.processMetrics,
		processorhelper.WithCapabilities(processorCapabilities))
}
