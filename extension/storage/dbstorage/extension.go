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

package dbstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/dbstorage"

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.uber.org/zap"
)

type databaseStorage struct {
	driverName     string
	datasourceName string
	logger         *zap.Logger
	db             *sql.DB
}

// Ensure this storage extension implements the appropriate interface
var _ storage.Extension = (*databaseStorage)(nil)

func newDBStorage(logger *zap.Logger, config *Config) (component.Extension, error) {
	return &databaseStorage{
		driverName:     config.DriverName,
		datasourceName: config.DataSource,
		logger:         logger,
	}, nil
}

// Start opens a connection to the database
func (ds *databaseStorage) Start(context.Context, component.Host) error {
	db, err := sql.Open(ds.driverName, ds.datasourceName)
	if err != nil {
		return err
	}

	if err := db.Ping(); err != nil {
		return err
	}
	ds.db = db
	return nil
}

// Shutdown closes the connection to the database
func (ds *databaseStorage) Shutdown(context.Context) error {
	return ds.db.Close()
}

// GetClient returns a storage client for an individual component
func (ds *databaseStorage) GetClient(ctx context.Context, kind component.Kind, ent component.ID, name string) (storage.Client, error) {
	var fullName string
	if name == "" {
		fullName = fmt.Sprintf("%s_%s_%s", kindString(kind), ent.Type(), ent.Name())
	} else {
		fullName = fmt.Sprintf("%s_%s_%s_%s", kindString(kind), ent.Type(), ent.Name(), name)
	}
	fullName = strings.ReplaceAll(fullName, " ", "")
	return newClient(ctx, ds.db, fullName)
}

func kindString(k component.Kind) string {
	switch k {
	case component.KindReceiver:
		return "receiver"
	case component.KindProcessor:
		return "processor"
	case component.KindExporter:
		return "exporter"
	case component.KindExtension:
		return "extension"
	default:
		return "other" // not expected
	}
}
