package registry

import (
	"fmt"
	"sort"

	"api-orchestrator/internal/config"
	"api-orchestrator/internal/fetcher"
)

// SourceInfo contains information about a source for external consumption.
type SourceInfo struct {
	Name          string            `json:"name"`
	URL           string            `json:"url"`
	Method        string            `json:"method"`
	DefaultParams map[string]string `json:"default_params"`
	Params        []string          `json:"params"`
	CacheTTL      int               `json:"cache_ttl_seconds"`
}

// Registry stores available sources by name.
type Registry struct {
	sources map[string]fetcher.Source
	configs map[string]config.SourceConfig
}

func New() *Registry {
	return &Registry{
		sources: make(map[string]fetcher.Source),
		configs: make(map[string]config.SourceConfig),
	}
}

func (r *Registry) Register(source fetcher.Source, cfg ...config.SourceConfig) {
	if source == nil {
		return
	}

	r.sources[source.Name()] = source
	if len(cfg) > 0 {
		r.configs[source.Name()] = cfg[0]
	}
}

func (r *Registry) Get(name string) (fetcher.Source, error) {
	source, ok := r.sources[name]
	if !ok {
		return nil, fmt.Errorf("source %q not found", name)
	}

	return source, nil
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.sources))

	for name := range r.sources {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

// List returns information about all registered sources.
func (r *Registry) List() []SourceInfo {
	list := make([]SourceInfo, 0, len(r.sources))

	for name := range r.sources {
		info := SourceInfo{
			Name: name,
		}

		if cfg, ok := r.configs[name]; ok {
			info.URL = cfg.URL
			info.Method = cfg.Method
			info.DefaultParams = cfg.DefaultParams
			info.CacheTTL = cfg.CacheTTLSeconds

			params := make([]string, 0)
			for key := range cfg.DefaultParams {
				params = append(params, key)
			}
			sort.Strings(params)
			info.Params = params
		}

		list = append(list, info)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})

	return list
}
