package requests

import (
	"bytes"
	"fmt"
	"net/http"
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
//   - b: The format string for the event data
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
// Returns the event value for data events, or an error for unknown event types.
func (s *ServerSentEvents) Read(b []byte) ([]byte, error) {
	name, value, _ := bytes.Cut(bytes.TrimRight(b, "\n"), []byte(":"))
	switch string(name) {
	case "":
		// An empty line in the for ": something" is a comment and should be ignored.
		// An empty line in the form ": something" is a comment and should be ignored.
		return nil, nil
	case "event":
		return nil, nil
	case "data":
		return value, nil
	default:
		return nil, fmt.Errorf("unknown event: %s", name)
	}
}

// SSE returns a middleware function that wraps an http.Handler to support Server-Sent Events.
// It sets the appropriate headers for SSE streaming and creates a new ServerSentEvents instance
// for handling the response.
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
