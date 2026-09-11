package main

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"api-orchestrator/internal/cache"
	"api-orchestrator/internal/config"
	"api-orchestrator/internal/fetcher"
	"api-orchestrator/internal/orchestrator"
	"api-orchestrator/internal/publisher"
	"api-orchestrator/internal/registry"
	"api-orchestrator/internal/scheduler"
)

//go:embed web/*
var webContent embed.FS

// fetchResponse is the unified JSON response returned by /fetch.
type fetchResponse struct {
	Results      map[string]any    `json:"results"`
	Errors       map[string]string `json:"errors,omitempty"`
	Timing       map[string]string `json:"timing,omitempty"`
	Cache        map[string]bool   `json:"cache,omitempty"`
	CircuitState map[string]string `json:"circuit_state,omitempty"`
}

// writeJSON sends a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// parseSourceNames extracts deduplicated source names from query parameters.
func parseSourceNames(r *http.Request) []string {
	var names []string
	seen := make(map[string]struct{})
	addName := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		names = append(names, value)
	}
	sourcesParam := r.URL.Query().Get("sources")
	if sourcesParam != "" {
		for _, part := range strings.Split(sourcesParam, ",") {
			addName(part)
		}
		return names
	}
	singleSource := r.URL.Query().Get("source")
	if singleSource != "" {
		addName(singleSource)
	}
	return names
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	webFS, err := fs.Sub(webContent, "web")
	if err != nil {
		logger.Error("failed to load web content", "error", err)
		os.Exit(1)
	}
	webHandler := http.FileServer(http.FS(webFS))

	cfg, err := config.Load("configs/sources.json")
	if err != nil {
		logger.Error("failed to load sources config", "error", err)
		os.Exit(1)
	}

	sources := registry.New()
	configsMap := make(map[string]config.SourceConfig)
	for _, sourceConfig := range cfg.Sources {
		sources.Register(fetcher.NewHTTPSource(sourceConfig), sourceConfig)
		configsMap[sourceConfig.Name] = sourceConfig
		logger.Info("registered source", "name", sourceConfig.Name)
	}

	appCache := cache.NewMemory()
	orch := orchestrator.New(sources, configsMap, 10*time.Second, appCache)

	// Publishing targets are optional: without the file the scheduler stays off.
	targetsCfg, err := config.LoadTargets("configs/targets.json")
	if err != nil {
		logger.Warn("failed to load targets config, scheduler disabled", "error", err)
		targetsCfg = &config.TargetsConfig{}
	}

	publishers := map[string]publisher.Publisher{
		"log": publisher.NewLog(logger),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sched := scheduler.New(targetsCfg.Targets, orch, publishers, logger)
	sched.Start(ctx)

	mux := http.NewServeMux()
	mux.Handle("/", webHandler)
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error": "method not allowed",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "pong",
		})
	})
	mux.HandleFunc("/cache/stats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error": "method not allowed",
			})
			return
		}
		writeJSON(w, http.StatusOK, orch.CacheStats())
	})
	mux.HandleFunc("/cache/clear", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error": "method not allowed, use POST",
			})
			return
		}
		orch.ClearCache()
		logger.Info("cache cleared")
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "cache cleared",
		})
	})
	mux.HandleFunc("/sources/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error": "method not allowed",
			})
			return
		}
		writeJSON(w, http.StatusOK, orch.CircuitBreakerStates())
	})
	mux.HandleFunc("/sources/list", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error": "method not allowed",
			})
			return
		}
		writeJSON(w, http.StatusOK, sources.List())
	})
	mux.HandleFunc("/fetch", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error": "method not allowed",
			})
			return
		}
		sourceNames := parseSourceNames(r)
		if len(sourceNames) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "query parameter 'sources' or 'source' is required",
			})
			return
		}
		params := map[string]string{}
		for key, values := range r.URL.Query() {
			if key == "source" || key == "sources" {
				continue
			}
			if len(values) > 0 {
				params[key] = values[0]
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		rawResults := orch.FetchMany(ctx, sourceNames, params)
		response := fetchResponse{
			Results:      make(map[string]any),
			Errors:       make(map[string]string),
			Timing:       make(map[string]string),
			Cache:        make(map[string]bool),
			CircuitState: make(map[string]string),
		}
		for name, result := range rawResults {
			response.Timing[name] = result.Duration
			response.Cache[name] = result.FromCache
			response.CircuitState[name] = result.CBState
			if result.Error != "" {
				response.Errors[name] = result.Error
				logger.Warn("source failed",
					"source", name,
					"error", result.Error,
					"duration", result.Duration,
					"circuit_state", result.CBState)
				continue
			}
			response.Results[name] = result.Data
			if result.FromCache {
				logger.Info("source served from cache",
					"source", name,
					"duration", result.Duration)
			} else {
				logger.Info("source succeeded",
					"source", name,
					"duration", result.Duration,
					"circuit_state", result.CBState)
			}
		}
		logger.Info("request completed",
			"sources", strings.Join(sourceNames, ","),
			"total_duration", time.Since(start).String(),
			"success_count", len(response.Results),
			"error_count", len(response.Errors))
		writeJSON(w, http.StatusOK, response)
	})
	mux.HandleFunc("/publish/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error": "method not allowed, use POST",
			})
			return
		}
		targetName := strings.TrimPrefix(r.URL.Path, "/publish/")
		if targetName == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "target name is required",
			})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		payload, err := sched.RunOnce(ctx, targetName)
		if err != nil {
			logger.Warn("on-demand publish failed", "target", targetName, "error", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error": err.Error(),
			})
			return
		}
		logger.Info("on-demand publish succeeded", "target", targetName)
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "published",
			"target":  targetName,
			"payload": payload,
		})
	})

		// Read the PORT environment variable (required by cloud providers like Render/Fly.io).
	// If not set, default to 8080 for local development.
	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("server is starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped with error", "error", err)
			os.Exit(1)
		}
	}()

	sig := <-quit
	logger.Info("shutdown signal received", "signal", sig.String())

	// Stop background publishers first, then drain in-flight HTTP requests.
	cancel()
	sched.Stop(10 * time.Second)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped gracefully")
}
