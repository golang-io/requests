package requests

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"
)

// ServerSentEvents implements http.Handler interface for Server-Sent Events (SSE) streaming.
// It wraps a http.ResponseWriter to provide SSE-specific functionality.
type ServerSentEvents struct {
	w http.ResponseWriter
}

// WriteHeader implements http.ResponseWriter interface.
// It writes the HTTP status code to the response.
func (s *ServerSentEvents) WriteHeader(statusCode int) {
	s.w.WriteHeader(statusCode)
}

// Write implements http.ResponseWriter interface.
// It writes the byte slice as a data event to the SSE stream.
func (s *ServerSentEvents) Write(b []byte) (int, error) {
	return s.Send("data", b)
}

// Header implements http.ResponseWriter interface.
// It returns the header map that will be sent by WriteHeader.
func (s *ServerSentEvents) Header() http.Header {
	return s.w.Header()
}

// Send writes a named SSE event with formatted data to the stream.
// It automatically flushes the response after writing.
// Parameters:
//   - name: The event name (e.g., "data", "event", etc.)
//   - b: The byte slice containing the event data
func (s *ServerSentEvents) Send(name string, b []byte) (int, error) {
	defer s.w.(http.Flusher).Flush()
	return s.w.Write([]byte(name + ":" + string(b) + "\n"))
}

// End terminates the SSE stream by writing two newlines.
// This should be called when the stream is complete.
func (s *ServerSentEvents) End() {
	_, _ = s.Write([]byte("\n\n"))
}

// Read parses an SSE message from the given byte slice.
// It handles different types of SSE events (empty, event, data).
// Returns:
//   - For data events: returns the event value
//   - For empty or event lines: returns nil, nil
//   - For unknown events: returns nil and an error
func (s *ServerSentEvents) Read(b []byte) ([]byte, error) {
	name, value, _ := bytes.Cut(bytes.TrimRight(b, "\n"), []byte(":"))
	switch string(name) {
	case "":
		// Empty lines or comments (": something") should be ignored
		return nil, nil
	case "event":
		// Event type declarations are processed but not returned
		return nil, nil
	case "data":
		// Data events return their value
		return value, nil
	default:
		// Unknown event types return an error
		return nil, fmt.Errorf("unknown event: %s", name)
	}
}

// SSE returns a middleware function that enables Server-Sent Events support.
// The middleware:
//   - Sets appropriate SSE headers (Content-Type, Cache-Control, etc.)
//   - Creates a ServerSentEvents wrapper for the response writer
//   - Ensures proper stream termination via deferred End() call
//   - Enables CORS support for cross-origin requests
func SSE() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sse := &ServerSentEvents{w: w}
			defer sse.End()
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			next.ServeHTTP(sse, r)
		})
	}
}

func CORS() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// printHandler creates a middleware for printing HTTP server request and response information.
// It records the request processing time and related statistics.
func printHandler(f func(ctx context.Context, stat *Stat)) func(handler http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := NewResponseWriter(w)
			buf, body, _ := CopyBody(r.Body)
			r.Body = body
			next.ServeHTTP(ww, r)
			f(r.Context(), serveLoad(ww, r, start, buf))
		})
	}
}
