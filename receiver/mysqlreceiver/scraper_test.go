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

package mysqlreceiver

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
)

func TestScrape(t *testing.T) {
	t.Run("successful scrape", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Username = "otel"
		cfg.Password = "otel"
		cfg.NetAddr = confignet.NetAddr{Endpoint: "localhost:3306"}
		cfg.Metrics.MysqlStatementEventCount.Enabled = true
		cfg.Metrics.MysqlStatementEventWaitTime.Enabled = true
		cfg.Metrics.MysqlConnectionErrors.Enabled = true
		cfg.Metrics.MysqlMysqlxWorkerThreads.Enabled = true
		cfg.Metrics.MysqlJoins.Enabled = true
		cfg.Metrics.MysqlTableOpenCache.Enabled = true
		cfg.Metrics.MysqlQueryClientCount.Enabled = true
		cfg.Metrics.MysqlQueryCount.Enabled = true
		cfg.Metrics.MysqlQuerySlowCount.Enabled = true

		cfg.Metrics.MysqlTableLockWaitReadCount.Enabled = true
		cfg.Metrics.MysqlTableLockWaitReadTime.Enabled = true
		cfg.Metrics.MysqlTableLockWaitWriteCount.Enabled = true
		cfg.Metrics.MysqlTableLockWaitWriteTime.Enabled = true

		cfg.Metrics.MysqlClientNetworkIo.Enabled = true

		scraper := newMySQLScraper(componenttest.NewNopReceiverCreateSettings(), cfg)
		scraper.sqlclient = &mockClient{
			globalStatsFile:             "global_stats",
			innodbStatsFile:             "innodb_stats",
			tableIoWaitsFile:            "table_io_waits_stats",
			indexIoWaitsFile:            "index_io_waits_stats",
			statementEventsFile:         "statement_events",
			tableLockWaitEventStatsFile: "table_lock_wait_event_stats",
		}

		actualMetrics, err := scraper.scrape(context.Background())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "expected.json")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, scrapertest.CompareMetrics(actualMetrics, expectedMetrics))
	})

	t.Run("scrape has partial failure", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Username = "otel"
		cfg.Password = "otel"
		cfg.NetAddr = confignet.NetAddr{Endpoint: "localhost:3306"}

		cfg.Metrics.MysqlTableLockWaitReadCount.Enabled = true
		cfg.Metrics.MysqlTableLockWaitReadTime.Enabled = true
		cfg.Metrics.MysqlTableLockWaitWriteCount.Enabled = true
		cfg.Metrics.MysqlTableLockWaitWriteTime.Enabled = true

		scraper := newMySQLScraper(componenttest.NewNopReceiverCreateSettings(), cfg)
		scraper.sqlclient = &mockClient{
			globalStatsFile:             "global_stats_partial",
			innodbStatsFile:             "innodb_stats_empty",
			tableIoWaitsFile:            "table_io_waits_stats_empty",
			indexIoWaitsFile:            "index_io_waits_stats_empty",
			statementEventsFile:         "statement_events_empty",
			tableLockWaitEventStatsFile: "table_lock_wait_event_stats_empty",
		}

		actualMetrics, scrapeErr := scraper.scrape(context.Background())
		require.Error(t, scrapeErr)

		expectedFile := filepath.Join("testdata", "scraper", "expected_partial.json")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)
		assert.NoError(t, scrapertest.CompareMetrics(actualMetrics, expectedMetrics))

		var partialError scrapererror.PartialScrapeError
		require.True(t, errors.As(scrapeErr, &partialError), "returned error was not PartialScrapeError")
		// 5 comes from 4 failed "must-have" metrics that aren't present,
		// and the other failure comes from a row that fails to parse as a number
		require.Equal(t, partialError.Failed, 5, "Expected partial error count to be 5")
	})

}

var _ client = (*mockClient)(nil)

type mockClient struct {
	globalStatsFile             string
	innodbStatsFile             string
	tableIoWaitsFile            string
	indexIoWaitsFile            string
	statementEventsFile         string
	tableLockWaitEventStatsFile string
}

func readFile(fname string) (map[string]string, error) {
	var stats = map[string]string{}
	file, err := os.Open(filepath.Join("testdata", "scraper", fname+".txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := strings.Split(scanner.Text(), "\t")
		stats[text[0]] = text[1]
	}
	return stats, nil
}

func (c *mockClient) Connect() error {
	return nil
}

func (c *mockClient) getGlobalStats() (map[string]string, error) {
	return readFile(c.globalStatsFile)
}

func (c *mockClient) getInnodbStats() (map[string]string, error) {
	return readFile(c.innodbStatsFile)
}

func (c *mockClient) getTableIoWaitsStats() ([]TableIoWaitsStats, error) {
	var stats []TableIoWaitsStats
	file, err := os.Open(filepath.Join("testdata", "scraper", c.tableIoWaitsFile+".txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var s TableIoWaitsStats
		text := strings.Split(scanner.Text(), "\t")

		s.schema = text[0]
		s.name = text[1]
		s.countDelete, _ = parseInt(text[2])
		s.countFetch, _ = parseInt(text[3])
		s.countInsert, _ = parseInt(text[4])
		s.countUpdate, _ = parseInt(text[5])
		s.timeDelete, _ = parseInt(text[6])
		s.timeFetch, _ = parseInt(text[7])
		s.timeInsert, _ = parseInt(text[8])
		s.timeUpdate, _ = parseInt(text[9])

		stats = append(stats, s)
	}
	return stats, nil
}

func (c *mockClient) getIndexIoWaitsStats() ([]IndexIoWaitsStats, error) {
	var stats []IndexIoWaitsStats
	file, err := os.Open(filepath.Join("testdata", "scraper", c.indexIoWaitsFile+".txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var s IndexIoWaitsStats
		text := strings.Split(scanner.Text(), "\t")

		s.schema = text[0]
		s.name = text[1]
		s.index = text[2]
		s.countDelete, _ = parseInt(text[3])
		s.countFetch, _ = parseInt(text[4])
		s.countInsert, _ = parseInt(text[5])
		s.countUpdate, _ = parseInt(text[6])
		s.timeDelete, _ = parseInt(text[7])
		s.timeFetch, _ = parseInt(text[8])
		s.timeInsert, _ = parseInt(text[9])
		s.timeUpdate, _ = parseInt(text[10])

		stats = append(stats, s)
	}
	return stats, nil
}

func (c *mockClient) getStatementEventsStats() ([]StatementEventStats, error) {
	var stats []StatementEventStats
	file, err := os.Open(filepath.Join("testdata", "scraper", c.statementEventsFile+".txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var s StatementEventStats
		text := strings.Split(scanner.Text(), "\t")

		s.schema = text[0]
		s.digest = text[1]
		s.digestText = text[2]
		s.sumTimerWait, _ = parseInt(text[3])
		s.countErrors, _ = parseInt(text[4])
		s.countWarnings, _ = parseInt(text[5])
		s.countRowsAffected, _ = parseInt(text[6])
		s.countRowsSent, _ = parseInt(text[7])
		s.countRowsExamined, _ = parseInt(text[8])
		s.countCreatedTmpDiskTables, _ = parseInt(text[9])
		s.countCreatedTmpTables, _ = parseInt(text[10])
		s.countSortMergePasses, _ = parseInt(text[11])
		s.countSortRows, _ = parseInt(text[12])
		s.countNoIndexUsed, _ = parseInt(text[13])

		stats = append(stats, s)
	}
	return stats, nil
}

func (c *mockClient) getTableLockWaitEventStats() ([]tableLockWaitEventStats, error) {
	var stats []tableLockWaitEventStats
	file, err := os.Open(filepath.Join("testdata", "scraper", c.tableLockWaitEventStatsFile+".txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var s tableLockWaitEventStats
		text := strings.Split(scanner.Text(), "\t")

		s.schema = text[0]
		s.name = text[1]
		s.countReadNormal, _ = parseInt(text[2])
		s.countReadWithSharedLocks, _ = parseInt(text[3])
		s.countReadHighPriority, _ = parseInt(text[4])
		s.countReadNoInsert, _ = parseInt(text[5])
		s.countReadExternal, _ = parseInt(text[6])
		s.countWriteAllowWrite, _ = parseInt(text[7])
		s.countWriteConcurrentInsert, _ = parseInt(text[8])
		s.countWriteLowPriority, _ = parseInt(text[9])
		s.countWriteNormal, _ = parseInt(text[10])
		s.countWriteExternal, _ = parseInt(text[11])
		s.sumTimerReadNormal, _ = parseInt(text[12])
		s.sumTimerReadWithSharedLocks, _ = parseInt(text[13])
		s.sumTimerReadHighPriority, _ = parseInt(text[14])
		s.sumTimerReadNoInsert, _ = parseInt(text[15])
		s.sumTimerReadExternal, _ = parseInt(text[16])
		s.sumTimerWriteAllowWrite, _ = parseInt(text[17])
		s.sumTimerWriteConcurrentInsert, _ = parseInt(text[18])
		s.sumTimerWriteLowPriority, _ = parseInt(text[19])
		s.sumTimerWriteNormal, _ = parseInt(text[20])
		s.sumTimerWriteExternal, _ = parseInt(text[21])

		stats = append(stats, s)
	}
	return stats, nil
}

func (c *mockClient) Close() error {
	return nil
}
