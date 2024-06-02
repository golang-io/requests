package requests

import (
	"bufio"
	"net"
	"net/http"
)

// ResponseWriter wrap `http.ResponseWriter` interface.
type ResponseWriter struct {
	http.ResponseWriter
	wroteHeader bool
	StatusCode  int
	Content     []byte
}

func (w *ResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.StatusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *ResponseWriter) Write(buf []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	n, err := w.ResponseWriter.Write(buf)
	w.Content = append(w.Content, buf...)
	return n, err
}

func (w *ResponseWriter) Flush() {
	w.wroteHeader = true
	w.ResponseWriter.(http.Flusher).Flush()
}

func (w *ResponseWriter) Push(target string, opts *http.PushOptions) error {
	return w.ResponseWriter.(http.Pusher).Push(target, opts)
}

func (w *ResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj := w.ResponseWriter.(http.Hijacker)
	return hj.Hijack()
}
