package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"api-orchestrator/internal/cache"
	"api-orchestrator/internal/circuitbreaker"
	"api-orchestrator/internal/config"
	"api-orchestrator/internal/parser"
	"api-orchestrator/internal/registry"
)

// Result contains either mapped source data or an error message.
type Result struct {
	Data      any    `json:"data,omitempty"`
	Error     string `json:"error,omitempty"`
	Duration  string `json:"duration,omitempty"`
	FromCache bool   `json:"from_cache"`
	CBState   string `json:"circuit_breaker_state,omitempty"`
}

// Orchestrator runs multiple sources concurrently.
type Orchestrator struct {
	registry        *registry.Registry
	configs         map[string]config.SourceConfig
	timeout         time.Duration
	cache           cache.Cache
	circuitBreakers map[string]*circuitbreaker.CircuitBreaker
	mu              sync.RWMutex
}

func New(reg *registry.Registry, configs map[string]config.SourceConfig, timeout time.Duration, c cache.Cache) *Orchestrator {
	breakers := make(map[string]*circuitbreaker.CircuitBreaker)

	for name, cfg := range configs {
		breakers[name] = circuitbreaker.New(
			cfg.CircuitBreaker.FailureThreshold,
			cfg.CircuitBreaker.SuccessThreshold,
			time.Duration(cfg.CircuitBreaker.RecoveryTimeoutSeconds)*time.Second,
		)
	}

	return &Orchestrator{
		registry:        reg,
		configs:         configs,
		timeout:         timeout,
		cache:           c,
		circuitBreakers: breakers,
	}
}

// FetchMany starts one goroutine per source and waits for all results.
func (o *Orchestrator) FetchMany(
	ctx context.Context,
	names []string,
	params map[string]string,
) map[string]Result {
	results := make(map[string]Result, len(names))

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, name := range names {
		wg.Add(1)

		go func(sourceName string) {
			defer wg.Done()

			sourceCtx, cancel := context.WithTimeout(ctx, o.timeout)
			defer cancel()

			result := o.fetchOne(sourceCtx, sourceName, params)

			mu.Lock()
			results[sourceName] = result
			mu.Unlock()
		}(name)
	}

	wg.Wait()

	return results
}

func (o *Orchestrator) fetchOne(
	ctx context.Context,
	name string,
	params map[string]string,
) Result {
	start := time.Now()

	o.mu.RLock()
	cb := o.circuitBreakers[name]
	o.mu.RUnlock()

	cbState := cb.State().String()

	source, err := o.registry.Get(name)
	if err != nil {
		return Result{
			Error:    err.Error(),
			Duration: time.Since(start).String(),
			CBState:  cbState,
		}
	}

	cfg, ok := o.configs[name]
	if !ok {
		return Result{
			Error:    "source config not found",
			Duration: time.Since(start).String(),
			CBState:  cbState,
		}
	}

	cacheKey := o.buildCacheKey(name, params)

	if cachedData, found := o.cache.Get(cacheKey); found {
		mappedData, err := parser.ApplyMapping(cachedData, cfg.ResponseMapping)
		if err == nil {
			return Result{
				Data:      mappedData,
				Duration:  time.Since(start).String(),
				FromCache: true,
				CBState:   cbState,
			}
		}
	}

	if err := cb.Allow(); err != nil {
		return Result{
			Error:    fmt.Sprintf("circuit breaker is open for source %q", name),
			Duration: time.Since(start).String(),
			CBState:  "open",
		}
	}

	data, err := source.Fetch(ctx, params)
	if err != nil {
		cb.RecordFailure()
		return Result{
			Error:    err.Error(),
			Duration: time.Since(start).String(),
			CBState:  cb.State().String(),
		}
	}

	if !json.Valid(data) {
		cb.RecordFailure()
		return Result{
			Error:    "source returned invalid JSON",
			Duration: time.Since(start).String(),
			CBState:  cb.State().String(),
		}
	}

	cb.RecordSuccess()

	if cfg.CacheTTLSeconds > 0 {
		ttl := time.Duration(cfg.CacheTTLSeconds) * time.Second
		o.cache.Set(cacheKey, data, ttl)
	}

	mappedData, err := parser.ApplyMapping(data, cfg.ResponseMapping)
	if err != nil {
		return Result{
			Error:    fmt.Sprintf("mapping failed: %v", err),
			Duration: time.Since(start).String(),
			CBState:  cb.State().String(),
		}
	}

	return Result{
		Data:      mappedData,
		Duration:  time.Since(start).String(),
		FromCache: false,
		CBState:   cb.State().String(),
	}
}

// buildCacheKey builds a DETERMINISTIC cache key:
// default and request params are merged, keys are sorted,
// so the same logical request always produces the same key.
func (o *Orchestrator) buildCacheKey(sourceName string, params map[string]string) string {
	merged := make(map[string]string)

	if cfg, ok := o.configs[sourceName]; ok {
		for key, value := range cfg.DefaultParams {
			merged[key] = value
		}
	}

	for key, value := range params {
		merged[key] = value
	}

	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := sourceName
	for _, key := range keys {
		result += "|" + key + "=" + merged[key]
	}

	return result
}

// ClearCache clears the entire cache.
func (o *Orchestrator) ClearCache() {
	o.cache.Clear()
}

// CacheStats returns cache statistics.
func (o *Orchestrator) CacheStats() map[string]int {
	return o.cache.Stats()
}

// CircuitBreakerStates returns the state of all circuit breakers.
func (o *Orchestrator) CircuitBreakerStates() map[string]string {
	o.mu.RLock()
	defer o.mu.RUnlock()

	states := make(map[string]string)
	for name, cb := range o.circuitBreakers {
		states[name] = cb.State().String()
	}
	return states
}
