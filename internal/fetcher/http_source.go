package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"api-orchestrator/internal/config"
	"api-orchestrator/internal/ratelimit"
	"api-orchestrator/internal/retry"
)

// HTTPSource is a generic source that performs an HTTP request
// based on configuration.
type HTTPSource struct {
	cfg      config.SourceConfig
	client   *http.Client
	limiter  *ratelimit.Limiter
	retryCfg retry.Config
}

func NewHTTPSource(cfg config.SourceConfig) *HTTPSource {
	timeout := cfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 10
	}

	limiter := ratelimit.New(cfg.RateLimit.RequestsPerSecond)

	retryableCodes := make(map[int]bool)
	for _, code := range cfg.Retry.RetryableStatusCodes {
		retryableCodes[code] = true
	}

	retryCfg := retry.Config{
		MaxAttempts:          cfg.Retry.MaxAttempts,
		InitialDelay:         time.Duration(cfg.Retry.InitialDelayMs) * time.Millisecond,
		MaxDelay:             time.Duration(cfg.Retry.MaxDelayMs) * time.Millisecond,
		RetryableStatusCodes: retryableCodes,
	}

	if retryCfg.MaxAttempts <= 0 {
		retryCfg.MaxAttempts = 1
	}

	return &HTTPSource{
		cfg: cfg,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
		limiter:  limiter,
		retryCfg: retryCfg,
	}
}

func (s *HTTPSource) Name() string {
	return s.cfg.Name
}

func (s *HTTPSource) Fetch(ctx context.Context, params map[string]string) ([]byte, error) {
	mergedParams := make(map[string]string)

	for key, value := range s.cfg.DefaultParams {
		mergedParams[key] = value
	}

	for key, value := range params {
		mergedParams[key] = value
	}

	requestURL, err := buildURL(s.cfg.URL, mergedParams)
	if err != nil {
		return nil, err
	}

	method := strings.ToUpper(s.cfg.Method)
	if method == "" {
		method = http.MethodGet
	}

	var resultBody []byte

	err = retry.Do(ctx, s.retryCfg, func() (int, error) {
		if err := s.limiter.Wait(ctx); err != nil {
			return 0, err
		}

		req, err := http.NewRequestWithContext(ctx, method, requestURL, nil)
		if err != nil {
			return 0, err
		}

		for key, value := range s.cfg.Headers {
			req.Header.Set(key, value)
		}

		resp, err := s.client.Do(req)
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return resp.StatusCode, err
		}

		if resp.StatusCode >= http.StatusBadRequest {
			return resp.StatusCode, fmt.Errorf("source returned status %d", resp.StatusCode)
		}

		resultBody = body
		return resp.StatusCode, nil
	})

	if err != nil {
		return nil, err
	}

	return resultBody, nil
}

func buildURL(template string, params map[string]string) (string, error) {
	result := template

	for key, value := range params {
		placeholder := "{" + key + "}"

		if strings.Contains(result, placeholder) {
			result = strings.ReplaceAll(
				result,
				placeholder,
				url.QueryEscape(value),
			)
		}
	}

	if strings.Contains(result, "{") {
		return "", fmt.Errorf("missing required URL parameter in %q", result)
	}

	return result, nil
}
