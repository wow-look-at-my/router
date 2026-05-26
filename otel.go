package router

import (
	"net/http"

	"go.opentelemetry.io/otel/trace"
)

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

func startSpan(tracer trace.Tracer, req *http.Request, routePattern string) (*http.Request, trace.Span, *statusWriter) {
	if tracer == nil {
		return req, nil, nil
	}

	ctx, span := tracer.Start(req.Context(), req.Method+" "+routePattern)
	return req.WithContext(ctx), span, nil
}
