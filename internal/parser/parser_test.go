package parser

import (
	"reflect"
	"testing"
)

func TestApplyMapping(t *testing.T) {
	cases := []struct {
		name    string
		json    string
		mapping map[string]string
		want    map[string]any
		wantErr bool
	}{
		{
			name: "top-level simple field",
			json: `{"temperature": 14.5}`,
			mapping: map[string]string{
				"temp": "temperature",
			},
			want: map[string]any{
				"temp": 14.5,
			},
		},
		{
			name: "nested path via dot notation",
			json: `{"current":{"temperature_2m":14.5,"weather_code":2}}`,
			mapping: map[string]string{
				"temperature":  "current.temperature_2m",
				"weather_code": "current.weather_code",
			},
			want: map[string]any{
				"temperature":  14.5,
				"weather_code": float64(2),
			},
		},
		{
			name: "mapping an entire object",
			json: `{"rates":{"USD":1.0,"EUR":0.85}}`,
			mapping: map[string]string{
				"rates": "rates",
			},
			want: map[string]any{
				"rates": map[string]any{
					"USD": 1.0,
					"EUR": 0.85,
				},
			},
		},
		{
			name: "string value",
			json: `{"current":{"time":"2026-09-04T00:30"}}`,
			mapping: map[string]string{
				"time": "current.time",
			},
			want: map[string]any{
				"time": "2026-09-04T00:30",
			},
		},
		{
			name: "non-existent path returns error",
			json: `{"current":{"temperature":14}}`,
			mapping: map[string]string{
				"temp": "missing.path",
			},
			wantErr: true,
		},
		{
			name:    "empty mapping returns full object (passthrough)",
			json:    `{"a":1}`,
			mapping: map[string]string{},
			want: map[string]any{
				"a": float64(1),
			},
		},
		{
			name:    "invalid JSON returns error",
			json:    `{not valid json`,
			mapping: map[string]string{"x": "y"},
			wantErr: true,
		},
		{
			name: "real-world scenario: Open-Meteo",
			json: `{"current":{"temperature_2m":14.5,"weather_code":3,"time":"2026-09-04T22:00"}}`,
			mapping: map[string]string{
				"temperature":  "current.temperature_2m",
				"weather_code": "current.weather_code",
				"time":         "current.time",
			},
			want: map[string]any{
				"temperature":  14.5,
				"weather_code": float64(3),
				"time":         "2026-09-04T22:00",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ApplyMapping([]byte(tc.json), tc.mapping)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, but got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ApplyMapping = %+v, want %+v", got, tc.want)
			}
		})
	}
}