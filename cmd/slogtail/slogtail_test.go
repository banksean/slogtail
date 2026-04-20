package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestHandlerHandleFormatsStructuredLog(t *testing.T) {
	var out bytes.Buffer
	handler := NewHandler(nil, &out)
	handler.colorize = false

	input := map[string]any{
		slog.LevelKey:   "INFO",
		slog.TimeKey:    "2026-04-20T12:34:56.789Z",
		slog.MessageKey: "started",
		"user":          "sam",
		"count":         float64(2),
	}

	err := handler.Handle(context.Background(), input)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	ts, err := time.Parse(time.RFC3339Nano, "2026-04-20T12:34:56.789Z")
	if err != nil {
		t.Fatalf("time.Parse() error = %v", err)
	}

	attrs, err := json.MarshalIndent(map[string]any{
		"count": float64(2),
		"user":  "sam",
	}, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent() error = %v", err)
	}

	want := ts.Local().Format(timeFormat) + " INFO: started " + string(attrs) + "\n"
	if got := out.String(); got != want {
		t.Fatalf("Handle() output = %q, want %q", got, want)
	}
}

func TestHandlerHandleUsesReplaceAttr(t *testing.T) {
	var out bytes.Buffer
	handler := NewHandler(&slog.HandlerOptions{
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.LevelKey {
				return slog.String(slog.LevelKey, "NOTICE")
			}
			return attr
		},
	}, &out)
	handler.colorize = false

	err := handler.Handle(context.Background(), map[string]any{
		slog.LevelKey:   "INFO",
		slog.MessageKey: "rewritten",
	})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if got := out.String(); got != "NOTICE: rewritten {}\n" {
		t.Fatalf("Handle() output = %q, want %q", got, "NOTICE: rewritten {}\n")
	}
}

func TestHandlerHandleErrors(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]any
		want  string
	}{
		{
			name:  "missing level",
			input: map[string]any{slog.MessageKey: "no level"},
			want:  "level is not a string",
		},
		{
			name:  "unknown level",
			input: map[string]any{slog.LevelKey: "TRACE"},
			want:  `unknown level name "TRACE"`,
		},
		{
			name: "marshal failure",
			input: map[string]any{
				slog.LevelKey: "INFO",
				"bad":         make(chan int),
			},
			want: "error when marshaling attrs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			handler := NewHandler(nil, &out)
			handler.colorize = false

			err := handler.Handle(context.Background(), tt.input)
			if err == nil {
				t.Fatalf("Handle() error = nil, want %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Handle() error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}
}

func TestProcessLineWritesAndFlushes(t *testing.T) {
	writer := &stubWriteFlusher{}
	handler := NewHandler(nil, writer)
	handler.colorize = false

	var stderr bytes.Buffer
	processLine(
		context.Background(),
		handler,
		writer,
		`{"level":"INFO","msg":"ok","request_id":"abc123"}`,
		&stderr,
	)

	if writer.flushCount != 1 {
		t.Fatalf("Flush() calls = %d, want 1", writer.flushCount)
	}
	if got := writer.String(); got != "INFO: ok {\n  \"request_id\": \"abc123\"\n}\n" {
		t.Fatalf("writer output = %q, want %q", got, "INFO: ok {\n  \"request_id\": \"abc123\"\n}\n")
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("stderr output = %q, want empty", got)
	}
}

func TestProcessLineReportsErrors(t *testing.T) {
	writer := &stubWriteFlusher{flushErr: errors.New("flush failed")}
	handler := NewHandler(nil, writer)
	handler.colorize = false

	var stderr bytes.Buffer
	processLine(context.Background(), handler, writer, "{", &stderr)

	if writer.flushCount != 1 {
		t.Fatalf("Flush() calls = %d, want 1", writer.flushCount)
	}
	got := stderr.String()
	for _, want := range []string{
		"unexpected EOF",
		"level is not a string",
		"flush failed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stderr output = %q, want substring %q", got, want)
		}
	}
}

type stubWriteFlusher struct {
	bytes.Buffer
	flushCount int
	flushErr   error
}

func (s *stubWriteFlusher) Flush() error {
	s.flushCount++
	return s.flushErr
}
