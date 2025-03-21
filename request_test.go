package requests

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func Test_makeBody(t *testing.T) {
	tests := []struct {
		name    string
		body    any
		wantErr bool
	}{
		{
			name: "nil body",
			body: nil,
		},
		{
			name: "byte slice body",
			body: []byte("test data"),
		},
		{
			name: "string body",
			body: "test data",
		},
		{
			name: "bytes buffer body",
			body: bytes.NewBuffer([]byte("test data")),
		},
		{
			name: "io.Reader body",
			body: strings.NewReader("test data"),
		},
		{
			name: "url.Values body",
			body: url.Values{"key": {"value"}},
		},
		{
			name: "struct body",
			body: struct {
				Name string `json:"name"`
			}{"test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := makeBody(tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("makeBody() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.body == nil {
				if reader != nil {
					t.Error("expected nil reader for nil body")
				}
				return
			}
			if reader == nil {
				t.Error("expected non-nil reader")
				return
			}
		})
	}
}

func TestNewRequestWithContext(t *testing.T) {
	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
	}{
		{
			name: "basic request",
			opts: []Option{MethodGet, URL("http://example.com")},
		},
		{
			name: "request with path",
			opts: []Option{MethodGet, URL("http://example.com"), Path("/api"), Path("/v1")},
		},
		{
			name: "request with query",
			opts: []Option{MethodGet, URL("http://example.com"), Param("key", "value")},
		},
		{
			name: "request with headers",
			opts: []Option{MethodGet, URL("http://example.com"), Header("Content-Type", "application/json")},
		},
		{
			name: "request with cookies",
			opts: []Option{MethodGet, URL("http://example.com"), Cookie(http.Cookie{Name: "session", Value: "123"})},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := newOptions(tt.opts)
			req, err := NewRequestWithContext(context.Background(), options)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRequestWithContext() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if req == nil {
				t.Error("expected non-nil request")
				return
			}
			// Verify request properties
			if req.Method != options.Method {
				t.Errorf("expected method %s, got %s", options.Method, req.Method)
			}
			if !strings.HasPrefix(req.URL.String(), options.URL) {
				t.Errorf("expected URL to start with %s, got %s", options.URL, req.URL.String())
			}
		})
	}
}
