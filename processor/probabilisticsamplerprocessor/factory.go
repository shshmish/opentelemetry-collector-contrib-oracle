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

package probabilisticsamplerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"

import (
	"context"
	"sync"

	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/consumer"
)

const (
	// The value of "type" trace-samplers in configuration.
	typeStr = "probabilistic_sampler"
	// The stability level of the processor.
	stability = component.StabilityLevelBeta
)

var onceMetrics sync.Once

// NewFactory returns a new factory for the Probabilistic sampler processor.
func NewFactory() component.ProcessorFactory {
	onceMetrics.Do(func() {
		// TODO: Handle this err
		_ = view.Register(SamplingProcessorMetricViews(configtelemetry.LevelNormal)...)
	})

	return component.NewProcessorFactory(
		typeStr,
		createDefaultConfig,
		component.WithTracesProcessor(createTracesProcessor, stability))
}

func createDefaultConfig() component.ProcessorConfig {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(component.NewID(typeStr)),
	}
}

// createTracesProcessor creates a trace processor based on this config.
func createTracesProcessor(
	ctx context.Context,
	set component.ProcessorCreateSettings,
	cfg component.ProcessorConfig,
	nextConsumer consumer.Traces,
) (component.TracesProcessor, error) {
	return newTracesProcessor(ctx, set, cfg.(*Config), nextConsumer)
}
