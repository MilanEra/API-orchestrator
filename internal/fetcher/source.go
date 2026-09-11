package fetcher

import "context"

// Source describes any data source that can return data.
type Source interface {
	Name() string
	Fetch(ctx context.Context, params map[string]string) ([]byte, error)
}
