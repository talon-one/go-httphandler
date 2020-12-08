package httphandler

import (
	"net/http"

	"go.uber.org/atomic"
)

// make sure *SafeResponseWriter implements http.ResponseWriter.
var _ http.ResponseWriter = &safeResponseWriter{}

type safeResponseWriter struct {
	written *atomic.Bool
	writer  http.ResponseWriter
}

func (w *safeResponseWriter) Header() http.Header {
	return w.writer.Header()
}

func (w *safeResponseWriter) Write(bytes []byte) (int, error) {
	w.written.Store(true)
	return w.writer.Write(bytes)
}

func (w *safeResponseWriter) WriteHeader(statusCode int) {
	if w.written.Load() {
		return
	}
	w.written.Store(true)
	w.writer.WriteHeader(statusCode)
}

func (w *safeResponseWriter) Written() bool {
	return w.written.Load()
}

func newSafeResponseWriter(writer http.ResponseWriter) *safeResponseWriter {
	return &safeResponseWriter{
		written: atomic.NewBool(false),
		writer:  writer,
	}
}
