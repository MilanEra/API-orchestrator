package parser

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ApplyMapping extracts values from raw JSON according to the mapping.
// Mapping format: "new_field": "path.to.field"
func ApplyMapping(data []byte, mapping map[string]string) (map[string]any, error) {
	if len(mapping) == 0 {
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, err
		}
		return raw, nil
	}

	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	result := make(map[string]any, len(mapping))

	for newField, path := range mapping {
		value, err := extractValue(raw, path)
		if err != nil {
			return nil, fmt.Errorf("failed to extract %q: %w", path, err)
		}
		result[newField] = value
	}

	return result, nil
}

func extractValue(data any, path string) (any, error) {
	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		if current == nil {
			return nil, fmt.Errorf("path %q not found", path)
		}

		switch v := current.(type) {
		case map[string]any:
			value, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("key %q not found in path %q", part, path)
			}
			current = value
		default:
			return nil, fmt.Errorf("expected map at %q, got %T", path, current)
		}
	}

	return current, nil
}
