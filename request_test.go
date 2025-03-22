package requests

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestMakeBody(t *testing.T) {
	tests := []struct {
		name    string
		body    any
		want    string
		wantErr bool
	}{
		{
			name: "nil body",
			body: nil,
			want: "",
		},
		{
			name: "byte slice",
			body: []byte("test data"),
			want: "test data",
		},
		{
			name: "string",
			body: "test string",
			want: "test string",
		},
		{
			name: "bytes.Buffer pointer",
			body: bytes.NewBuffer([]byte("buffer data")),
			want: "buffer data",
		},
		{
			name: "strings.Reader",
			body: strings.NewReader("reader data"),
			want: "reader data",
		},
		{
			name: "url.Values",
			body: url.Values{"key": {"value"}},
			want: "key=value",
		},
		{
			name: "func returning ReadCloser",
			body: func() (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("func data")), nil
			},
			want: "func data",
		},
		{
			name: "struct to JSON",
			body: struct {
				Key string `json:"key"`
			}{Key: "value"},
			want: `{"key":"value"}`,
		},
		{
			name: "error func",
			body: func() (io.ReadCloser, error) {
				return nil, io.EOF
			},
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			body:    make(chan int),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := makeBody(tt.body)

			if tt.wantErr {
				if err == nil {
					t.Error("期望错误但得到 nil")
				}
				return
			}

			if err != nil {
				t.Errorf("makeBody() 错误 = %v", err)
				return
			}

			if reader == nil && tt.want != "" {
				t.Error("期望非空 reader 但得到 nil")
				return
			}

			if reader != nil {
				got, err := io.ReadAll(reader)
				if err != nil {
					t.Errorf("读取 body 失败: %v", err)
					return
				}

				if string(got) != tt.want {
					t.Errorf("makeBody() = %v, 期望 %v", string(got), tt.want)
				}
			}
		})
	}
}

func TestNewRequestWithContext(t *testing.T) {
	tests := []struct {
		name    string
		opts    []Option
		want    func(*http.Request) bool
		wantErr bool
	}{
		{
			name: "基本请求",
			opts: []Option{MethodGet, URL("http://example.com")},
			want: func(r *http.Request) bool {
				return r.Method == "GET" &&
					r.URL.String() == "http://example.com"
			},
		},
		{
			name: "带路径参数",
			opts: []Option{MethodGet, URL("http://example.com"), Path("/api"), Path("/v1")},
			want: func(r *http.Request) bool {
				return r.URL.Path == "/api/v1"
			},
		},
		{
			name: "带查询参数",
			opts: []Option{MethodGet, URL("http://example.com"), Param("key", "value")},

			want: func(r *http.Request) bool {
				return r.URL.RawQuery == "key=value"
			},
		},
		{
			name: "带请求头",
			opts: []Option{MethodGet, URL("http://example.com"), Header("X-Test", "test-value")},
			want: func(r *http.Request) bool {
				return r.Header.Get("X-Test") == "test-value"
			},
		},
		{
			name: "带Cookie",
			opts: []Option{MethodGet, URL("http://example.com"), Cookie(http.Cookie{Name: "session", Value: "123"})},
			want: func(r *http.Request) bool {
				cookies := r.Cookies()
				return len(cookies) == 1 &&
					cookies[0].Name == "session" &&
					cookies[0].Value == "123"
			},
		},
		{
			name:    "无效URL",
			opts:    []Option{MethodGet, URL("://invalid"), Cookie(http.Cookie{Name: "session", Value: "123"})},
			wantErr: true,
		},
		{
			name:    "无效body",
			opts:    []Option{MethodGet, URL("http://example.com"), Body(make(chan int))},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			req, err := NewRequestWithContext(ctx, newOptions(tt.opts))

			if tt.wantErr {
				if err == nil {
					t.Error("期望错误但得到 nil")
				}
				return
			}

			if err != nil {
				t.Errorf("NewRequestWithContext() 错误 = %v", err)
				return
			}

			if !tt.want(req) {
				t.Errorf("请求不符合预期条件")
			}
		})
	}
}
