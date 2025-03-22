package requests

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestStreamRoundTrip 测试streamRoundTrip中间件
func TestStreamRoundTrip(t *testing.T) {
	// 创建一个模拟的响应体
	responseBody := "line1\nline2\nline3"
	expectedLines := []string{"line1", "line2", "line3"}

	// 创建一个模拟的RoundTripper
	mockTransport := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
		}, nil
	})

	// 创建请求
	req, _ := http.NewRequest("GET", "http://example.com", nil)

	// 收集处理的行
	var lines []string
	middleware := streamRoundTrip(func(_ int64, data []byte) error {
		lines = append(lines, string(bytes.TrimRight(data, "\n")))
		return nil
	})

	// 执行请求
	resp, err := middleware(mockTransport).RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}

	// 验证响应
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// 验证处理的行
	if len(lines) != len(expectedLines) {
		t.Errorf("Expected %d lines, got %d", len(expectedLines), len(lines))
	}
	for i, line := range expectedLines {
		if lines[i] != line {
			t.Errorf("Line %d: expected '%s', got '%s'", i, line, lines[i])
		}
	}
}

// BenchmarkStreamRead 性能测试
func BenchmarkStreamRead(b *testing.B) {
	// 准备测试数据
	var testData strings.Builder
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&testData, "Line %d\n", i)
	}
	data := testData.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(data)
		_, err := streamRead(reader, func(_ int64, _ []byte) error {
			return nil
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}
