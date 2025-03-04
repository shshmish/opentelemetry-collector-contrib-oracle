// Copyright  OpenTelemetry Authors
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

package apachereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachereceiver"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachereceiver/internal/metadata"
)

const (
	readmeURL                         = "https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/apachereceiver/README.md"
	EmitServerNameAsResourceAttribute = "receiver.apache.emitServerNameAsResourceAttribute"
	EmitPortAsResourceAttribute       = "receiver.apache.emitPortAsResourceAttribute"
)

func init() {
	featuregate.GetRegistry().MustRegisterID(
		EmitServerNameAsResourceAttribute,
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("When enabled, the name of the server will be sent as an apache.server.name resource attribute instead of a metric-level server_name attribute."),
	)
	featuregate.GetRegistry().MustRegisterID(
		EmitPortAsResourceAttribute,
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("When enabled, the port of the server will be sent as an apache.server.port resource attribute."),
	)
}

type apacheScraper struct {
	settings   component.TelemetrySettings
	cfg        *Config
	httpClient *http.Client
	mb         *metadata.MetricsBuilder
	serverName string
	port       string

	// Feature gates regarding resource attributes
	emitMetricsWithServerNameAsResourceAttribute bool
	emitMetricsWithPortAsResourceAttribute       bool
}

func newApacheScraper(
	settings component.ReceiverCreateSettings,
	cfg *Config,
	serverName string,
	port string,
) *apacheScraper {
	a := &apacheScraper{
		settings:   settings.TelemetrySettings,
		cfg:        cfg,
		mb:         metadata.NewMetricsBuilder(cfg.Metrics, settings.BuildInfo),
		serverName: serverName,
		port:       port,
		emitMetricsWithServerNameAsResourceAttribute: featuregate.GetRegistry().IsEnabled(EmitServerNameAsResourceAttribute),
		emitMetricsWithPortAsResourceAttribute:       featuregate.GetRegistry().IsEnabled(EmitPortAsResourceAttribute),
	}

	if !a.emitMetricsWithServerNameAsResourceAttribute {
		settings.Logger.Warn(
			fmt.Sprintf("Feature gate %s is not enabled. Please see the README for more information: %s", EmitServerNameAsResourceAttribute, readmeURL),
		)
	}

	if !a.emitMetricsWithPortAsResourceAttribute {
		settings.Logger.Warn(
			fmt.Sprintf("Feature gate %s is not enabled. Please see the README for more information: %s", EmitPortAsResourceAttribute, readmeURL),
		)
	}

	return a
}

func (r *apacheScraper) start(_ context.Context, host component.Host) error {
	httpClient, err := r.cfg.ToClient(host, r.settings)
	if err != nil {
		return err
	}
	r.httpClient = httpClient
	return nil
}

func (r *apacheScraper) scrape(context.Context) (pmetric.Metrics, error) {
	if r.httpClient == nil {
		return pmetric.Metrics{}, errors.New("failed to connect to Apache HTTPd")
	}

	stats, err := r.GetStats()
	if err != nil {
		r.settings.Logger.Error("failed to fetch Apache Httpd stats", zap.Error(err))
		return pmetric.Metrics{}, err
	}

	emitWith := []metadata.ResourceMetricsOption{}

	if r.emitMetricsWithServerNameAsResourceAttribute {
		err = r.scrapeWithoutServerNameAttr(stats)
		emitWith = append(emitWith, metadata.WithApacheServerName(r.serverName))
	} else {
		err = r.scrapeWithServerNameAttr(stats)
	}

	if r.emitMetricsWithPortAsResourceAttribute {
		emitWith = append(emitWith, metadata.WithApacheServerPort(r.port))
	}

	return r.mb.Emit(emitWith...), err
}

func (r *apacheScraper) scrapeWithServerNameAttr(stats string) error {
	errs := &scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now())
	for metricKey, metricValue := range parseStats(stats) {
		switch metricKey {
		case "ServerUptimeSeconds":
			addPartialIfError(errs, r.mb.RecordApacheUptimeDataPointWithServerName(now, metricValue, r.serverName))
		case "ConnsTotal":
			addPartialIfError(errs, r.mb.RecordApacheCurrentConnectionsDataPointWithServerName(now, metricValue, r.serverName))
		case "BusyWorkers":
			addPartialIfError(errs, r.mb.RecordApacheWorkersDataPointWithServerName(now, metricValue, r.serverName,
				metadata.AttributeWorkersStateBusy))
		case "IdleWorkers":
			addPartialIfError(errs, r.mb.RecordApacheWorkersDataPointWithServerName(now, metricValue, r.serverName,
				metadata.AttributeWorkersStateIdle))
		case "Total Accesses":
			addPartialIfError(errs, r.mb.RecordApacheRequestsDataPointWithServerName(now, metricValue, r.serverName))
		case "Total kBytes":
			i, err := strconv.ParseInt(metricValue, 10, 64)
			if err != nil {
				errs.AddPartial(1, err)
			} else {
				r.mb.RecordApacheTrafficDataPointWithServerName(now, kbytesToBytes(i), r.serverName)
			}
		case "CPUChildrenSystem":
			addPartialIfError(
				errs,
				r.mb.RecordApacheCPUTimeDataPointWithServerName(now, metricValue, r.serverName, metadata.AttributeCPULevelChildren, metadata.AttributeCPUModeSystem),
			)
		case "CPUChildrenUser":
			addPartialIfError(
				errs,
				r.mb.RecordApacheCPUTimeDataPointWithServerName(now, metricValue, r.serverName, metadata.AttributeCPULevelChildren, metadata.AttributeCPUModeUser),
			)
		case "CPUSystem":
			addPartialIfError(
				errs,
				r.mb.RecordApacheCPUTimeDataPointWithServerName(now, metricValue, r.serverName, metadata.AttributeCPULevelSelf, metadata.AttributeCPUModeSystem),
			)
		case "CPUUser":
			addPartialIfError(
				errs,
				r.mb.RecordApacheCPUTimeDataPointWithServerName(now, metricValue, r.serverName, metadata.AttributeCPULevelSelf, metadata.AttributeCPUModeUser),
			)
		case "CPULoad":
			addPartialIfError(errs, r.mb.RecordApacheCPULoadDataPointWithServerName(now, metricValue, r.serverName))
		case "Load1":
			addPartialIfError(errs, r.mb.RecordApacheLoad1DataPointWithServerName(now, metricValue, r.serverName))
		case "Load5":
			addPartialIfError(errs, r.mb.RecordApacheLoad5DataPointWithServerName(now, metricValue, r.serverName))
		case "Load15":
			addPartialIfError(errs, r.mb.RecordApacheLoad15DataPointWithServerName(now, metricValue, r.serverName))
		case "Total Duration":
			addPartialIfError(errs, r.mb.RecordApacheRequestTimeDataPointWithServerName(now, metricValue, r.serverName))
		case "Scoreboard":
			scoreboardMap := parseScoreboard(metricValue)
			for state, score := range scoreboardMap {
				r.mb.RecordApacheScoreboardDataPointWithServerName(now, score, r.serverName, state)
			}
		}
	}

	return errs.Combine()
}

func (r *apacheScraper) scrapeWithoutServerNameAttr(stats string) error {
	errs := &scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now())
	for metricKey, metricValue := range parseStats(stats) {
		switch metricKey {
		case "ServerUptimeSeconds":
			addPartialIfError(errs, r.mb.RecordApacheUptimeDataPoint(now, metricValue))
		case "ConnsTotal":
			addPartialIfError(errs, r.mb.RecordApacheCurrentConnectionsDataPoint(now, metricValue))
		case "BusyWorkers":
			addPartialIfError(errs, r.mb.RecordApacheWorkersDataPoint(now, metricValue, metadata.AttributeWorkersStateBusy))
		case "IdleWorkers":
			addPartialIfError(errs, r.mb.RecordApacheWorkersDataPoint(now, metricValue, metadata.AttributeWorkersStateIdle))
		case "Total Accesses":
			addPartialIfError(errs, r.mb.RecordApacheRequestsDataPoint(now, metricValue))
		case "Total kBytes":
			i, err := strconv.ParseInt(metricValue, 10, 64)
			if err != nil {
				errs.AddPartial(1, err)
			} else {
				r.mb.RecordApacheTrafficDataPoint(now, kbytesToBytes(i))
			}
		case "CPUChildrenSystem":
			addPartialIfError(
				errs,
				r.mb.RecordApacheCPUTimeDataPoint(now, metricValue, metadata.AttributeCPULevelChildren, metadata.AttributeCPUModeSystem),
			)
		case "CPUChildrenUser":
			addPartialIfError(
				errs,
				r.mb.RecordApacheCPUTimeDataPoint(now, metricValue, metadata.AttributeCPULevelChildren, metadata.AttributeCPUModeUser),
			)
		case "CPUSystem":
			addPartialIfError(
				errs,
				r.mb.RecordApacheCPUTimeDataPoint(now, metricValue, metadata.AttributeCPULevelSelf, metadata.AttributeCPUModeSystem),
			)
		case "CPUUser":
			addPartialIfError(
				errs,
				r.mb.RecordApacheCPUTimeDataPoint(now, metricValue, metadata.AttributeCPULevelSelf, metadata.AttributeCPUModeUser),
			)
		case "CPULoad":
			addPartialIfError(errs, r.mb.RecordApacheCPULoadDataPoint(now, metricValue))
		case "Load1":
			addPartialIfError(errs, r.mb.RecordApacheLoad1DataPoint(now, metricValue))
		case "Load5":
			addPartialIfError(errs, r.mb.RecordApacheLoad5DataPoint(now, metricValue))
		case "Load15":
			addPartialIfError(errs, r.mb.RecordApacheLoad15DataPoint(now, metricValue))
		case "Total Duration":
			addPartialIfError(errs, r.mb.RecordApacheRequestTimeDataPoint(now, metricValue))
		case "Scoreboard":
			scoreboardMap := parseScoreboard(metricValue)
			for state, score := range scoreboardMap {
				r.mb.RecordApacheScoreboardDataPoint(now, score, state)
			}
		}
	}

	return errs.Combine()
}

func addPartialIfError(errs *scrapererror.ScrapeErrors, err error) {
	if err != nil {
		errs.AddPartial(1, err)
	}
}

// GetStats collects metric stats by making a get request at an endpoint.
func (r *apacheScraper) GetStats() (string, error) {
	resp, err := r.httpClient.Get(r.cfg.Endpoint)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// parseStats converts a response body key:values into a map.
func parseStats(resp string) map[string]string {
	metrics := make(map[string]string)

	fields := strings.Split(resp, "\n")
	for _, field := range fields {
		index := strings.Index(field, ": ")
		if index == -1 {
			continue
		}
		metrics[field[:index]] = field[index+2:]
	}
	return metrics
}

type scoreboardCountsByLabel map[metadata.AttributeScoreboardState]int64

// parseScoreboard quantifies the symbolic mapping of the scoreboard.
func parseScoreboard(values string) scoreboardCountsByLabel {
	scoreboard := scoreboardCountsByLabel{
		metadata.AttributeScoreboardStateWaiting:     0,
		metadata.AttributeScoreboardStateStarting:    0,
		metadata.AttributeScoreboardStateReading:     0,
		metadata.AttributeScoreboardStateSending:     0,
		metadata.AttributeScoreboardStateKeepalive:   0,
		metadata.AttributeScoreboardStateDnslookup:   0,
		metadata.AttributeScoreboardStateClosing:     0,
		metadata.AttributeScoreboardStateLogging:     0,
		metadata.AttributeScoreboardStateFinishing:   0,
		metadata.AttributeScoreboardStateIdleCleanup: 0,
		metadata.AttributeScoreboardStateOpen:        0,
	}

	for _, char := range values {
		switch string(char) {
		case "_":
			scoreboard[metadata.AttributeScoreboardStateWaiting]++
		case "S":
			scoreboard[metadata.AttributeScoreboardStateStarting]++
		case "R":
			scoreboard[metadata.AttributeScoreboardStateReading]++
		case "W":
			scoreboard[metadata.AttributeScoreboardStateSending]++
		case "K":
			scoreboard[metadata.AttributeScoreboardStateKeepalive]++
		case "D":
			scoreboard[metadata.AttributeScoreboardStateDnslookup]++
		case "C":
			scoreboard[metadata.AttributeScoreboardStateClosing]++
		case "L":
			scoreboard[metadata.AttributeScoreboardStateLogging]++
		case "G":
			scoreboard[metadata.AttributeScoreboardStateFinishing]++
		case "I":
			scoreboard[metadata.AttributeScoreboardStateIdleCleanup]++
		case ".":
			scoreboard[metadata.AttributeScoreboardStateOpen]++
		default:
			scoreboard[metadata.AttributeScoreboardStateUnknown]++
		}
	}
	return scoreboard
}

// kbytesToBytes converts 1 Kibibyte to 1024 bytes.
func kbytesToBytes(i int64) int64 {
	return 1024 * i
}
