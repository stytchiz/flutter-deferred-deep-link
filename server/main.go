// Copyright 2023 The Authors (see AUTHORS file)
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

// Package main implements a simple HTTP/JSON REST example.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/renderer"
	"github.com/abcxyz/pkg/serving"
)

const (
	defaultPort           = "8080"
	deferredDeeplinkTable = "DeferredDeepLinks"
)

var (
	port = flag.String("port", defaultPort, "Specifies server port to listen on.")
	db   *sql.DB
)

type AppQueryRequest struct {
	DeviceID string `json:"device_id"`
	Pill     string `json:"pill"`
}

func handleAppQuery(h *renderer.Renderer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := logging.FromContext(r.Context())
		logger.InfoContext(r.Context(), "handling request")
		decoder := json.NewDecoder(r.Body)
		var req AppQueryRequest
		if err := decoder.Decode(&req); err != nil {
			h.RenderJSON(w, http.StatusInternalServerError, fmt.Errorf("failed to unmarshal request: %v", err))
			return
		}

		if err := InsertRow(db, deferredDeeplinkTable, []string{req.DeviceID, req.Pill}); err != nil {
			h.RenderJSON(w, http.StatusInternalServerError, fmt.Errorf("failed to write to database: %v", err))
			return
		}

		h.RenderJSON(w, http.StatusOK, nil)
	})
}

// realMain creates an example backend HTTP server.
// This server supports graceful stopping and cancellation.
func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	// Connect to CloudSQL
	var err error
	db, err = connectWithConnector()
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}
	defer db.Close()

	// See https://github.com/go-sql-driver/mysql/issues/257#issuecomment-53886663.
	db.SetMaxIdleConns(0)
	db.SetMaxOpenConns(500)

	// Make a new renderer for rendering json.
	// Don't provide filesystem as we don't have templates to render.
	h, err := renderer.New(ctx, nil,
		renderer.WithOnError(func(err error) {
			logger.ErrorContext(ctx, "failed to render", "error", err)
		}))
	if err != nil {
		return fmt.Errorf("failed to create renderer for main server: %w", err)
	}

	r := chi.NewRouter()
	r.Mount("/", handleAppQuery(h))
	walkFunc := func(method, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		logger.DebugContext(ctx, "Route registered", "http_method", method, "route", route)
		return nil
	}

	if err := chi.Walk(r, walkFunc); err != nil {
		logger.ErrorContext(ctx, "error walking routes", "error", err)
	}

	httpServer := &http.Server{
		Addr:              *port,
		Handler:           r,
		ReadHeaderTimeout: 2 * time.Second,
	}

	logger.InfoContext(ctx, "starting server", "port", *port)
	server, err := serving.New(*port)
	if err != nil {
		return fmt.Errorf("error creating server: %w", err)
	}

	// This will block until the provided context is cancelled.
	if err := server.StartHTTP(ctx, httpServer); err != nil {
		return fmt.Errorf("error starting server: %w", err)
	}
	return nil
}

func main() {
	// creates a context that exits on interrupt signal.
	ctx, done := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer done()
	logger := logging.FromContext(ctx)

	flag.Parse()
	if err := realMain(logging.WithLogger(ctx, logger)); err != nil {
		done()
		logger.ErrorContext(ctx, err.Error())
		os.Exit(1)
	}
	logger.InfoContext(ctx, "completed")
}
