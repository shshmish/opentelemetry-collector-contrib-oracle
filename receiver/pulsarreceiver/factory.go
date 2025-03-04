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

package pulsarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
)

const (
	typeStr             = "pulsar"
	stability           = component.StabilityLevelAlpha
	defaultEncoding     = "otlp_proto"
	defaultTraceTopic   = "otlp_spans"
	defaultMeticsTopic  = "otlp_metrics"
	defaultLogsTopic    = "otlp_logs"
	defaultConsumerName = ""
	defaultSubscription = "otlp_subscription"
	defaultServiceURL   = "pulsar://localhost:6650"
)

// FactoryOption applies changes to PulsarExporterFactory.
type FactoryOption func(factory *pulsarReceiverFactory)

// WithTracesUnmarshalers adds Unmarshalers.
func WithTracesUnmarshalers(tracesUnmarshalers ...TracesUnmarshaler) FactoryOption {
	return func(factory *pulsarReceiverFactory) {
		for _, unmarshaler := range tracesUnmarshalers {
			factory.tracesUnmarshalers[unmarshaler.Encoding()] = unmarshaler
		}
	}
}

// WithMetricsUnmarshalers adds MetricsUnmarshalers.
func WithMetricsUnmarshalers(metricsUnmarshalers ...MetricsUnmarshaler) FactoryOption {
	return func(factory *pulsarReceiverFactory) {
		for _, unmarshaler := range metricsUnmarshalers {
			factory.metricsUnmarshalers[unmarshaler.Encoding()] = unmarshaler
		}
	}
}

// WithLogsUnmarshalers adds LogsUnmarshalers.
func WithLogsUnmarshalers(logsUnmarshalers ...LogsUnmarshaler) FactoryOption {
	return func(factory *pulsarReceiverFactory) {
		for _, unmarshaler := range logsUnmarshalers {
			factory.logsUnmarshalers[unmarshaler.Encoding()] = unmarshaler
		}
	}
}

// NewFactory creates Pulsar receiver factory.
func NewFactory(options ...FactoryOption) component.ReceiverFactory {

	f := &pulsarReceiverFactory{
		tracesUnmarshalers:  defaultTracesUnmarshalers(),
		metricsUnmarshalers: defaultMetricsUnmarshalers(),
		logsUnmarshalers:    defaultLogsUnmarshalers(),
	}
	for _, o := range options {
		o(f)
	}
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithTracesReceiver(f.createTracesReceiver, stability),
		component.WithMetricsReceiver(f.createMetricsReceiver, stability),
		component.WithLogsReceiver(f.createLogsReceiver, stability),
	)
}

type pulsarReceiverFactory struct {
	tracesUnmarshalers  map[string]TracesUnmarshaler
	metricsUnmarshalers map[string]MetricsUnmarshaler
	logsUnmarshalers    map[string]LogsUnmarshaler
}

func (f *pulsarReceiverFactory) createTracesReceiver(
	_ context.Context,
	set component.ReceiverCreateSettings,
	cfg component.ReceiverConfig,
	nextConsumer consumer.Traces,
) (component.TracesReceiver, error) {
	c := *(cfg.(*Config))
	if len(c.Topic) == 0 {
		c.Topic = defaultTraceTopic
	}
	r, err := newTracesReceiver(c, set, f.tracesUnmarshalers, nextConsumer)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (f *pulsarReceiverFactory) createMetricsReceiver(
	_ context.Context,
	set component.ReceiverCreateSettings,
	cfg component.ReceiverConfig,
	nextConsumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	c := *(cfg.(*Config))
	if len(c.Topic) == 0 {
		c.Topic = defaultMeticsTopic
	}
	r, err := newMetricsReceiver(c, set, f.metricsUnmarshalers, nextConsumer)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (f *pulsarReceiverFactory) createLogsReceiver(
	_ context.Context,
	set component.ReceiverCreateSettings,
	cfg component.ReceiverConfig,
	nextConsumer consumer.Logs,
) (component.LogsReceiver, error) {
	c := *(cfg.(*Config))
	if len(c.Topic) == 0 {
		c.Topic = defaultLogsTopic
	}
	r, err := newLogsReceiver(c, set, f.logsUnmarshalers, nextConsumer)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func createDefaultConfig() component.ReceiverConfig {
	return &Config{
		ReceiverSettings: config.NewReceiverSettings(component.NewID(typeStr)),
		Encoding:         defaultEncoding,
		ConsumerName:     defaultConsumerName,
		Subscription:     defaultSubscription,
		Endpoint:         defaultServiceURL,
	}
}
