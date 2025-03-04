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

package expvarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/expvarreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/expvarreceiver/internal/metadata"
)

const (
	typeStr         = "expvar"
	stability       = component.StabilityLevelBeta
	defaultPath     = "/debug/vars"
	defaultEndpoint = "http://localhost:8000" + defaultPath
	defaultTimeout  = 3 * time.Second
)

func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		newDefaultConfig,
		component.WithMetricsReceiver(newMetricsReceiver, stability))
}

func newMetricsReceiver(
	_ context.Context,
	set component.ReceiverCreateSettings,
	rCfg component.ReceiverConfig,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	cfg := rCfg.(*Config)

	expVar := newExpVarScraper(cfg, set)
	scraper, err := scraperhelper.NewScraper(
		typeStr,
		expVar.scrape,
		scraperhelper.WithStart(expVar.start),
	)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&cfg.ScraperControllerSettings,
		set,
		consumer,
		scraperhelper.AddScraper(scraper),
	)
}

func newDefaultConfig() component.ReceiverConfig {
	return &Config{
		ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(typeStr),
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: defaultEndpoint,
			Timeout:  defaultTimeout,
		},
		MetricsConfig: metadata.DefaultMetricsSettings(),
	}
}
