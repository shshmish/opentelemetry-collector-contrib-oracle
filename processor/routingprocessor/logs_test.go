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

package routingprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

func TestLogProcessorCapabilities(t *testing.T) {
	// prepare
	config := &Config{
		FromAttribute: "X-Tenant",
		Table: []RoutingTableItem{{
			Value:     "acme",
			Exporters: []string{"otlp"},
		}},
	}

	// test
	p := newLogProcessor(component.TelemetrySettings{Logger: zap.NewNop()}, config)
	require.NotNil(t, p)

	// verify
	assert.Equal(t, false, p.Capabilities().MutatesData)
}

func TestLogs_RoutingWorks_Context(t *testing.T) {
	defaultExp := &mockLogsExporter{}
	lExp := &mockLogsExporter{}

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[component.DataType]map[component.ID]component.Exporter {
			return map[component.DataType]map[component.ID]component.Exporter{
				component.DataTypeLogs: {
					component.NewID("otlp"):              defaultExp,
					component.NewIDWithName("otlp", "2"): lExp,
				},
			}
		},
	}

	exp := newLogProcessor(component.TelemetrySettings{Logger: zap.NewNop()}, &Config{
		FromAttribute:    "X-Tenant",
		AttributeSource:  contextAttributeSource,
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp/2"},
			},
		},
	})
	require.NoError(t, exp.Start(context.Background(), host))

	l := plog.NewLogs()
	rl := l.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("X-Tenant", "acme")

	t.Run("non default route is properly used", func(t *testing.T) {
		assert.NoError(t, exp.ConsumeLogs(
			metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
				"X-Tenant": "acme",
			})),
			l,
		))
		assert.Len(t, defaultExp.AllLogs(), 0,
			"log should not be routed to default exporter",
		)
		assert.Len(t, lExp.AllLogs(), 1,
			"log should be routed to non default exporter",
		)
	})

	t.Run("default route is taken when no matching route can be found", func(t *testing.T) {
		assert.NoError(t, exp.ConsumeLogs(
			metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
				"X-Tenant": "some-custom-value1",
			})),
			l,
		))
		assert.Len(t, defaultExp.AllLogs(), 1,
			"log should be routed to default exporter",
		)
		assert.Len(t, lExp.AllLogs(), 1,
			"log should not be routed to non default exporter",
		)
	})
}

func TestLogs_RoutingWorks_ResourceAttribute(t *testing.T) {
	defaultExp := &mockLogsExporter{}
	lExp := &mockLogsExporter{}

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[component.DataType]map[component.ID]component.Exporter {
			return map[component.DataType]map[component.ID]component.Exporter{
				component.DataTypeLogs: {
					component.NewID("otlp"):              defaultExp,
					component.NewIDWithName("otlp", "2"): lExp,
				},
			}
		},
	}

	exp := newLogProcessor(component.TelemetrySettings{Logger: zap.NewNop()}, &Config{
		FromAttribute:    "X-Tenant",
		AttributeSource:  resourceAttributeSource,
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp/2"},
			},
		},
	})
	require.NoError(t, exp.Start(context.Background(), host))

	t.Run("non default route is properly used", func(t *testing.T) {
		l := plog.NewLogs()
		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "acme")

		assert.NoError(t, exp.ConsumeLogs(context.Background(), l))
		assert.Len(t, defaultExp.AllLogs(), 0,
			"log should not be routed to default exporter",
		)
		assert.Len(t, lExp.AllLogs(), 1,
			"log should be routed to non default exporter",
		)
	})

	t.Run("default route is taken when no matching route can be found", func(t *testing.T) {
		l := plog.NewLogs()
		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "some-custom-value")

		assert.NoError(t, exp.ConsumeLogs(context.Background(), l))
		assert.Len(t, defaultExp.AllLogs(), 1,
			"log should be routed to default exporter",
		)
		assert.Len(t, lExp.AllLogs(), 1,
			"log should not be routed to non default exporter",
		)
	})
}

func TestLogs_RoutingWorks_ResourceAttribute_DropsRoutingAttribute(t *testing.T) {
	defaultExp := &mockLogsExporter{}
	lExp := &mockLogsExporter{}

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[component.DataType]map[component.ID]component.Exporter {
			return map[component.DataType]map[component.ID]component.Exporter{
				component.DataTypeLogs: {
					component.NewID("otlp"):              defaultExp,
					component.NewIDWithName("otlp", "2"): lExp,
				},
			}
		},
	}

	exp := newLogProcessor(component.TelemetrySettings{Logger: zap.NewNop()}, &Config{
		AttributeSource:              resourceAttributeSource,
		FromAttribute:                "X-Tenant",
		DropRoutingResourceAttribute: true,
		DefaultExporters:             []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp/2"},
			},
		},
	})
	require.NoError(t, exp.Start(context.Background(), host))

	l := plog.NewLogs()
	rm := l.ResourceLogs().AppendEmpty()
	rm.Resource().Attributes().PutStr("X-Tenant", "acme")
	rm.Resource().Attributes().PutStr("attr", "acme")

	assert.NoError(t, exp.ConsumeLogs(context.Background(), l))
	logs := lExp.AllLogs()
	require.Len(t, logs, 1, "log should be routed to non-default exporter")
	require.Equal(t, 1, logs[0].ResourceLogs().Len())
	attrs := logs[0].ResourceLogs().At(0).Resource().Attributes()
	_, ok := attrs.Get("X-Tenant")
	assert.False(t, ok, "routing attribute should have been dropped")
	v, ok := attrs.Get("attr")
	assert.True(t, ok, "non routing attributes shouldn't be dropped")
	assert.Equal(t, "acme", v.Str())
}

func TestLogs_AreCorrectlySplitPerResourceAttributeRouting(t *testing.T) {
	defaultExp := &mockLogsExporter{}
	lExp := &mockLogsExporter{}

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[component.DataType]map[component.ID]component.Exporter {
			return map[component.DataType]map[component.ID]component.Exporter{
				component.DataTypeLogs: {
					component.NewID("otlp"):              defaultExp,
					component.NewIDWithName("otlp", "2"): lExp,
				},
			}
		},
	}

	exp := newLogProcessor(component.TelemetrySettings{Logger: zap.NewNop()}, &Config{
		FromAttribute:    "X-Tenant",
		AttributeSource:  resourceAttributeSource,
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Value:     "acme",
				Exporters: []string{"otlp/2"},
			},
		},
	})

	l := plog.NewLogs()

	rl := l.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("X-Tenant", "acme")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	rl = l.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("X-Tenant", "acme")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	rl = l.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("X-Tenant", "something-else")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	ctx := context.Background()
	require.NoError(t, exp.Start(ctx, host))
	require.NoError(t, exp.ConsumeLogs(ctx, l))

	// The numbers below stem from the fact that data is routed and grouped
	// per resource attribute which is used for routing.
	// Hence the first 2 metrics are grouped together under one plog.Logs.
	assert.Len(t, defaultExp.AllLogs(), 1,
		"one log should be routed to default exporter",
	)
	assert.Len(t, lExp.AllLogs(), 1,
		"one log should be routed to non default exporter",
	)
}

func TestLogsAreCorrectlySplitPerResourceAttributeWithOTTL(t *testing.T) {
	defaultExp := &mockLogsExporter{}
	firstExp := &mockLogsExporter{}
	secondExp := &mockLogsExporter{}

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		GetExportersFunc: func() map[component.DataType]map[component.ID]component.Exporter {
			return map[component.DataType]map[component.ID]component.Exporter{
				component.DataTypeLogs: {
					component.NewID("otlp"):              defaultExp,
					component.NewIDWithName("otlp", "1"): firstExp,
					component.NewIDWithName("otlp", "2"): secondExp,
				},
			}
		},
	}

	exp := newLogProcessor(component.TelemetrySettings{Logger: zap.NewNop()}, &Config{
		DefaultExporters: []string{"otlp"},
		Table: []RoutingTableItem{
			{
				Statement: `route() where IsMatch(resource.attributes["X-Tenant"], ".*acme") == true`,
				Exporters: []string{"otlp/1"},
			},
			{
				Statement: `route() where IsMatch(resource.attributes["X-Tenant"], "_acme") == true`,
				Exporters: []string{"otlp/2"},
			},
		},
	})

	require.NoError(t, exp.Start(context.Background(), host))

	t.Run("logs matched by no expressions", func(t *testing.T) {
		defaultExp.Reset()
		firstExp.Reset()
		secondExp.Reset()

		l := plog.NewLogs()
		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "something-else")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		require.NoError(t, exp.ConsumeLogs(context.Background(), l))

		assert.Len(t, defaultExp.AllLogs(), 1)
		assert.Len(t, firstExp.AllLogs(), 0)
		assert.Len(t, secondExp.AllLogs(), 0)
	})

	t.Run("logs matched one of two expressions", func(t *testing.T) {
		defaultExp.Reset()
		firstExp.Reset()
		secondExp.Reset()

		l := plog.NewLogs()

		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "xacme")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		require.NoError(t, exp.ConsumeLogs(context.Background(), l))

		assert.Len(t, defaultExp.AllLogs(), 0)
		assert.Len(t, firstExp.AllLogs(), 1)
		assert.Len(t, secondExp.AllLogs(), 0)
	})

	t.Run("logs matched by all expressions", func(t *testing.T) {
		defaultExp.Reset()
		firstExp.Reset()
		secondExp.Reset()

		l := plog.NewLogs()

		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "x_acme")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		rl = l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "_acme")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		require.NoError(t, exp.ConsumeLogs(context.Background(), l))

		assert.Len(t, defaultExp.AllLogs(), 0)
		assert.Len(t, firstExp.AllLogs(), 1)
		assert.Len(t, secondExp.AllLogs(), 1)

		assert.Equal(t, firstExp.AllLogs()[0].LogRecordCount(), 2)
		assert.Equal(t, secondExp.AllLogs()[0].LogRecordCount(), 2)
		assert.Equal(t, firstExp.AllLogs(), secondExp.AllLogs())
	})

	t.Run("one log matched by all expressions, other matched none", func(t *testing.T) {
		defaultExp.Reset()
		firstExp.Reset()
		secondExp.Reset()

		l := plog.NewLogs()

		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "_acme")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		rl = l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("X-Tenant", "something-else")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		require.NoError(t, exp.ConsumeLogs(context.Background(), l))

		assert.Len(t, defaultExp.AllLogs(), 1)
		assert.Len(t, firstExp.AllLogs(), 1)
		assert.Len(t, secondExp.AllLogs(), 1)

		assert.Equal(t, firstExp.AllLogs(), secondExp.AllLogs())

		rspan := defaultExp.AllLogs()[0].ResourceLogs().At(0)
		attr, ok := rspan.Resource().Attributes().Get("X-Tenant")
		assert.True(t, ok, "routing attribute must exists")
		assert.Equal(t, attr.AsString(), "something-else")
	})
}

type mockLogsExporter struct {
	mockComponent
	consumertest.LogsSink
}
