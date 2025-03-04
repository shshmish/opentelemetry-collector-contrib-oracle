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

package bearertokenauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension"

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.uber.org/zap"
	"google.golang.org/grpc/credentials"
)

var _ credentials.PerRPCCredentials = (*PerRPCAuth)(nil)

// PerRPCAuth is a gRPC credentials.PerRPCCredentials implementation that returns an 'authorization' header.
type PerRPCAuth struct {
	metadata map[string]string
}

// GetRequestMetadata returns the request metadata to be used with the RPC.
func (c *PerRPCAuth) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return c.metadata, nil
}

// RequireTransportSecurity always returns true for this implementation. Passing bearer tokens in plain-text connections is a bad idea.
func (c *PerRPCAuth) RequireTransportSecurity() bool {
	return true
}

// BearerTokenAuth is an implementation of configauth.GRPCClientAuthenticator. It embeds a static authorization "bearer" token in every rpc call.
type BearerTokenAuth struct {
	muTokenString sync.RWMutex
	scheme        string
	tokenString   string

	shutdownCH chan struct{}

	filename string
	logger   *zap.Logger
}

var _ configauth.ClientAuthenticator = (*BearerTokenAuth)(nil)

func newBearerTokenAuth(cfg *Config, logger *zap.Logger) *BearerTokenAuth {
	if cfg.Filename != "" && cfg.BearerToken != "" {
		logger.Warn("a filename is specified. Configured token is ignored!")
	}
	return &BearerTokenAuth{
		scheme:      cfg.Scheme,
		tokenString: cfg.BearerToken,
		filename:    cfg.Filename,
		logger:      logger,
	}
}

// Start of BearerTokenAuth does nothing and returns nil if no filename
// is specified. Otherwise a routine is started to monitor the file containing
// the token to be transferred.
func (b *BearerTokenAuth) Start(ctx context.Context, host component.Host) error {
	if b.filename == "" {
		return nil
	}

	if b.shutdownCH != nil {
		return fmt.Errorf("bearerToken file monitoring is already running")
	}

	// Read file once
	b.refreshToken()

	b.shutdownCH = make(chan struct{})

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	// start file watcher
	go b.startWatcher(ctx, watcher)

	return watcher.Add(b.filename)
}

func (b *BearerTokenAuth) startWatcher(ctx context.Context, watcher *fsnotify.Watcher) {
	defer watcher.Close()
	for {
		select {
		case _, ok := <-b.shutdownCH:
			_ = ok
			return
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				continue
			}
			// NOTE: k8s configmaps uses symlinks, we need this workaround.
			// original configmap file is removed.
			// SEE: https://martensson.io/go-fsnotify-and-kubernetes-configmaps/
			if event.Op == fsnotify.Remove || event.Op == fsnotify.Chmod {
				// remove the watcher since the file is removed
				if err := watcher.Remove(event.Name); err != nil {
					b.logger.Error(err.Error())
				}
				// add a new watcher pointing to the new symlink/file
				if err := watcher.Add(b.filename); err != nil {
					b.logger.Error(err.Error())
				}
				b.refreshToken()
			}
			// also allow normal files to be modified and reloaded.
			if event.Op == fsnotify.Write {
				b.refreshToken()
			}
		}
	}
}

func (b *BearerTokenAuth) refreshToken() {
	b.logger.Info("refresh token", zap.String("filename", b.filename))
	token, err := os.ReadFile(b.filename)
	if err != nil {
		b.logger.Error(err.Error())
		return
	}
	b.muTokenString.Lock()
	b.tokenString = string(token)
	b.muTokenString.Unlock()
}

// Shutdown of BearerTokenAuth does nothing and returns nil
func (b *BearerTokenAuth) Shutdown(ctx context.Context) error {
	if b.filename == "" {
		return nil
	}

	if b.shutdownCH == nil {
		return fmt.Errorf("bearerToken file monitoring is not running")
	}
	b.shutdownCH <- struct{}{}
	close(b.shutdownCH)
	b.shutdownCH = nil
	return nil
}

// PerRPCCredentials returns PerRPCAuth an implementation of credentials.PerRPCCredentials that
func (b *BearerTokenAuth) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	return &PerRPCAuth{
		metadata: map[string]string{"authorization": b.bearerToken()},
	}, nil
}

func (b *BearerTokenAuth) bearerToken() string {
	b.muTokenString.RLock()
	token := fmt.Sprintf("%s %s", b.scheme, b.tokenString)
	b.muTokenString.RUnlock()
	return token
}

// RoundTripper is not implemented by BearerTokenAuth
func (b *BearerTokenAuth) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return &BearerAuthRoundTripper{
		baseTransport: base,
		bearerToken:   b.bearerToken(),
	}, nil
}

// BearerAuthRoundTripper intercepts and adds Bearer token Authorization headers to each http request.
type BearerAuthRoundTripper struct {
	baseTransport http.RoundTripper
	bearerToken   string
}

// RoundTrip modifies the original request and adds Bearer token Authorization headers.
func (interceptor *BearerAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	if req2.Header == nil {
		req2.Header = make(http.Header)
	}
	req2.Header.Set("Authorization", interceptor.bearerToken)
	return interceptor.baseTransport.RoundTrip(req2)
}
