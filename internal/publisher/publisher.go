package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"text/template"

	"api-orchestrator/internal/config"
)

// Publisher delivers prepared source data to an external destination.
// Implementations must be safe for concurrent use.
type Publisher interface {
	Publish(ctx context.Context, target config.TargetConfig, data map[string]any) error
}

// LogPublisher writes rendered payloads to the application log via slog.
// It is the default publisher for local development and debugging.
type LogPublisher struct {
	logger *slog.Logger
}

// NewLog creates a LogPublisher backed by the given logger.
func NewLog(logger *slog.Logger) *LogPublisher {
	return &LogPublisher{logger: logger}
}

// Publish renders the payload and writes it to the log.
func (p *LogPublisher) Publish(_ context.Context, target config.TargetConfig, data map[string]any) error {
	text, err := Render(target.Template, data)
	if err != nil {
		return err
	}
	p.logger.Info("published",
		"target", target.Name,
		"publisher", "log",
		"payload", text)
	return nil
}

// Render applies a text/template template to mapped data.
// An empty template falls back to compact JSON, so a target
// without a template still produces a readable payload.
func Render(tmpl string, data map[string]any) (string, error) {
	if tmpl == "" {
		b, err := json.Marshal(data)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	t, err := template.New("payload").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
