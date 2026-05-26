package router

import (
	"context"
	"net/http"
)

// Tracer creates spans for HTTP requests.
// Implement with your OpenTelemetry TracerProvider:
//
//	type otelTracer struct{ tracer trace.Tracer }
//	func (t otelTracer) Start(ctx context.Context, name string) (context.Context, Span) {
//	    ctx, span := t.tracer.Start(ctx, name)
//	    return ctx, span
//	}
type Tracer interface {
	Start(ctx context.Context, name string) (context.Context, Span)
}

// Span represents an active trace span. Compatible with
// go.opentelemetry.io/otel/trace.Span.
type Span interface {
	End()
}

// WithTracer sets the tracer used to create request spans.
// When not set, no spans are created.
func WithTracer(t Tracer) Option {
	return func(r *Router) {
		r.tracer = t
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wrote {
		w.status = code
		w.wrote = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.status = http.StatusOK
		w.wrote = true
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
