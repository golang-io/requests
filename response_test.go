package requests

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// TestNewResponse 测试newResponse函数的基本功能
func TestNewResponse(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com", nil)
	resp := newResponse(r)

	if resp.Request != r {
		t.Error("Request not set correctly")
	}
	if resp.Content == nil {
		t.Error("Content buffer not initialized")
	}
	if resp.StartAt.IsZero() {
		t.Error("StartAt not initialized")
	}
}

// TestResponseString 测试Response.String()方法
func TestResponseString(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com", nil)
	resp := newResponse(r)
	resp.Content.WriteString("test content")

	if resp.String() != "test content" {
		t.Errorf("Expected 'test content', got '%s'", resp.String())
	}
}

// TestResponseError 测试Response.Error()方法
func TestResponseError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"no error", nil, ""},
		{"with error", errors.New("test error"), "test error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := http.NewRequest("GET", "http://example.com", nil)
			resp := newResponse(r)
			resp.Err = tt.err
			if got := resp.Error(); got != tt.want {
				t.Errorf("Response.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestStreamRead 测试streamRead函数的基本功能
func TestStreamRead(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int64
		wantErr bool
	}{
		{"empty input", "", 0, false},
		{"single line", "hello\n", 6, false},
		{"multiple lines", "hello\nworld\n", 12, false},
		{"no newline at end", "hello", 5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			var lines []string

			gotLen, err := streamRead(reader, func(_ int64, data []byte) error {
				lines = append(lines, string(bytes.TrimRight(data, "\n")))
				return nil
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("streamRead() error = %v, wantErr %v", err, tt.wantErr)
			}
			if gotLen != tt.wantLen {
				t.Errorf("streamRead() length = %v, want %v", gotLen, tt.wantLen)
			}
		})
	}
}

// TestStreamReadError 测试streamRead函数的错误处理
func TestStreamReadError(t *testing.T) {
	// 测试回调函数返回错误的情况
	reader := strings.NewReader("test\ndata\n")
	expectedErr := errors.New("callback error")

	_, err := streamRead(reader, func(_ int64, _ []byte) error {
		return expectedErr
	})

	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	// 测试读取错误的情况
	errorReader := &errorReader{err: errors.New("read error")}
	_, err = streamRead(errorReader, func(_ int64, _ []byte) error {
		return nil
	})

	if err == nil || !strings.Contains(err.Error(), "read error") {
		t.Errorf("Expected read error, got %v", err)
	}
}

// 用于测试的错误读取器
type errorReader struct {
	err error
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, r.err
}
