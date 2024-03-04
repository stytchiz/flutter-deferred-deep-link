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
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/abcxyz/pkg/logging"
	"github.com/abcxyz/pkg/renderer"
	"github.com/abcxyz/pkg/serving"
)

const (
	defaultServerURL = "https://flutter-deferred-deep-link-23vgxprgkq-uc.a.run.app"
	defaultPort      = "8080"
)

var (
	port      = flag.String("port", defaultPort, "Specifies server port to listen on.")
	serverURL = flag.String("service_url", defaultServerURL, "Specifies server address.")

	db        *sql.DB
	templates = template.Must(template.ParseFiles("index.html"))
)

// empty
type TemplateParams struct{}

type DeferredDeepLinkQueryRequest struct {
	UserIP     string `json:"user_ip"`
	DeviceType string `json:"device_type"`
	Target     string `json:"target"`
}

func getClientIPFromHttpHeaders(header http.Header) (string, error) {
	// See https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For#syntax
	xForwardedFor := header.Get("X-Forwarded-For")
	if xForwardedFor == "" {
		return "", fmt.Errorf("X-Forwarded-For header is empty")
	}
	ips := strings.Split(xForwardedFor, ", ")
	if ips[0] == "" {
		return "", fmt.Errorf("client ip is empty")
	}
	return ips[0], nil
}

func handleAppQuery(h *renderer.Renderer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := logging.FromContext(r.Context())
		logger.InfoContext(r.Context(), "handling app request...but app is not installed")

		clientIP, err := getClientIPFromHttpHeaders(r.Header)
		if err != nil {
			h.RenderJSON(w, http.StatusBadRequest, fmt.Errorf("failed to read client headers: %v", err))
			return
		}

		target := chi.URLParam(r, "target")
		req := &DeferredDeepLinkQueryRequest{Target: target, UserIP: clientIP, DeviceType: "Android"}
		reqB, _ := json.Marshal(&req)
		logger.InfoContext(r.Context(), "calling service to add new deferred deep link entry")
		resp, err := http.Post(*serverURL+"/deferDeepLink", "application/json", bytes.NewBuffer(reqB))
		if err != nil {
			h.RenderJSON(w, http.StatusInternalServerError, fmt.Errorf("failed to make request: %v", err))
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			h.RenderJSON(w, http.StatusInternalServerError, fmt.Errorf("failed to decode response body: %v", err))
			return
		}

		logger.InfoContext(r.Context(), "got deep link API response", "body", string(body))

		h.RenderJSON(w, http.StatusOK, nil)
	})
}

func handleNewDeferredDeepLink(h *renderer.Renderer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := logging.FromContext(r.Context())
		logger.InfoContext(r.Context(), "handling new deferred deep link request")
		decoder := json.NewDecoder(r.Body)
		var req DeferredDeepLinkQueryRequest
		if err := decoder.Decode(&req); err != nil {
			h.RenderJSON(w, http.StatusBadRequest, fmt.Errorf("failed to unmarshal request: %v", err))
			return
		}
		if err := updateDatabaseForDeferredDeepLinkQuery(r.Context(), db, &req); err != nil {
			h.RenderJSON(w, http.StatusInternalServerError, fmt.Errorf("failed to write to database: %v", err))
			return
		}

		h.RenderJSON(w, http.StatusOK, nil)
	})
}

func handleDeferredDeepLinkQuery(h *renderer.Renderer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.RenderJSON(w, http.StatusOK, nil)
	})
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	renderHTML(w, "index")
}

func renderHTML(w http.ResponseWriter, name string) {
	err := templates.ExecuteTemplate(w, name+".html", &TemplateParams{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// realMain creates an example backend HTTP server.
// This server supports graceful stopping and cancellation.
func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	// Connect to CloudSQL instance
	var cleanup func() error
	db, cleanup = getDB()
	defer cleanup()
	logger.InfoContext(ctx, "connected to database server")

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
	r.Get("/", handleIndex)
	r.Mount("/app", handleAppQuery(h))
	r.Mount("/deferDeepLink", handleNewDeferredDeepLink(h))
	r.Mount("/queryDeferredDeepLinks", handleDeferredDeepLinkQuery(h))
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
		logger.ErrorContext(ctx, "server shut down with error: "+err.Error())
		os.Exit(1)
	}
	logger.InfoContext(ctx, "server has shut down successfully")
}
