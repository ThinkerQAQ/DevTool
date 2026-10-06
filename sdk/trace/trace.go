package trace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	fileEnv  = "DEVTOOL_TRACE_FILE"
	runIDEnv = "DEVTOOL_RUN_ID"
)

type contextKey struct{}

// Carrier is the metadata propagated across DevTool process boundaries.
// It deliberately contains no request payload, source, prompt, or credential data.
type Carrier struct {
	TraceID      string `json:"trace_id"`
	ParentSpanID string `json:"parent_span_id"`
}

// Attributes describe one timing span without recording operation payloads.
type Attributes struct {
	Name         string
	Layer        string
	Tool         string
	Service      string
	Provider     string
	Method       string
	RequestBytes int
}

type event struct {
	Timestamp     string  `json:"timestamp"`
	TraceID       string  `json:"trace_id"`
	SpanID        string  `json:"span_id"`
	ParentSpanID  string  `json:"parent_span_id"`
	Name          string  `json:"name"`
	Layer         string  `json:"layer"`
	Tool          string  `json:"tool,omitempty"`
	Service       string  `json:"service,omitempty"`
	Provider      string  `json:"provider,omitempty"`
	Method        string  `json:"method,omitempty"`
	DurationMS    float64 `json:"duration_ms"`
	Status        string  `json:"status"`
	RequestBytes  int     `json:"request_bytes"`
	ResponseBytes int     `json:"response_bytes"`
	ErrorClass    string  `json:"error_class,omitempty"`
	RunID         string  `json:"run_id"`
}

type writer struct {
	once sync.Once
	mu   sync.Mutex
	file *os.File
}

var processWriter writer
var fallbackID atomic.Uint64

// Span records a single JSONL event when tracing is enabled for the process.
type Span struct {
	enabled bool
	start   time.Time
	attrs   Attributes
	carrier Carrier
	parent  string
}

// Start creates a child span from ctx. Tracing is disabled when
// DEVTOOL_TRACE_FILE is empty, in which case Start returns ctx unchanged.
func Start(ctx context.Context, attrs Attributes) (context.Context, *Span) {
	if strings.TrimSpace(os.Getenv(fileEnv)) == "" {
		return ctx, &Span{}
	}

	parent, _ := ctx.Value(contextKey{}).(Carrier)
	traceID := parent.TraceID
	if traceID == "" {
		traceID = newID()
	}
	spanID := newID()
	carrier := Carrier{TraceID: traceID, ParentSpanID: spanID}
	return context.WithValue(ctx, contextKey{}, carrier), &Span{
		enabled: true,
		start:   time.Now(),
		attrs:   attrs,
		carrier: carrier,
		parent:  parent.ParentSpanID,
	}
}

// End completes the span. Writer failures are reported to stderr but never
// replace the operation result being measured.
func (s *Span) End(responseBytes int, err error) {
	if s == nil || !s.enabled {
		return
	}
	status := "ok"
	errorClass := ""
	if err != nil {
		status = "error"
		errorClass = classifyError(err)
	}
	entry := event{
		Timestamp:     s.start.UTC().Format(time.RFC3339Nano),
		TraceID:       s.carrier.TraceID,
		SpanID:        s.carrier.ParentSpanID,
		ParentSpanID:  s.parent,
		Name:          s.attrs.Name,
		Layer:         s.attrs.Layer,
		Tool:          s.attrs.Tool,
		Service:       s.attrs.Service,
		Provider:      s.attrs.Provider,
		Method:        s.attrs.Method,
		DurationMS:    float64(time.Since(s.start)) / float64(time.Millisecond),
		Status:        status,
		RequestBytes:  s.attrs.RequestBytes,
		ResponseBytes: responseBytes,
		ErrorClass:    errorClass,
		RunID:         strings.TrimSpace(os.Getenv(runIDEnv)),
	}
	if writeErr := processWriter.write(entry); writeErr != nil {
		fmt.Fprintf(os.Stderr, "devtool tracing: %v\n", writeErr)
	}
}

// CarrierFromContext returns metadata safe to send over a DevTool protocol
// boundary. It contains identifiers only.
func CarrierFromContext(ctx context.Context) *Carrier {
	carrier, ok := ctx.Value(contextKey{}).(Carrier)
	if !ok || carrier.TraceID == "" || carrier.ParentSpanID == "" {
		return nil
	}
	copy := carrier
	return &copy
}

// ContextWithCarrier restores a propagated parent for spans in another process.
func ContextWithCarrier(ctx context.Context, carrier *Carrier) context.Context {
	if carrier == nil || strings.TrimSpace(carrier.TraceID) == "" || strings.TrimSpace(carrier.ParentSpanID) == "" {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, *carrier)
}

func (w *writer) write(entry event) error {
	w.once.Do(func() {
		path := strings.TrimSpace(os.Getenv(fileEnv))
		if path == "" {
			return
		}
		if dir := filepath.Dir(path); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return
			}
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err == nil {
			w.file = file
		}
	})
	if w.file == nil {
		return fmt.Errorf("cannot open %s", fileEnv)
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err = w.file.Write(raw)
	return err
}

func classifyError(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	default:
		return "error"
	}
}

func newID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("%x-%x-%x", time.Now().UnixNano(), os.Getpid(), fallbackID.Add(1))
}
