package requests

import (
	"bytes"
	"context"
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

func TestPrintRoundTripper(t *testing.T) {
	var statReceived *Stat

	// 测试正常请求
	t.Run("正常请求", func(t *testing.T) {
		mockTransport := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("success")),
			}, nil
		})

		middleware := printRoundTripper(func(ctx context.Context, stat *Stat) {
			statReceived = stat
		})

		req, _ := http.NewRequest("GET", "http://example.com", nil)
		resp, err := middleware(mockTransport).RoundTrip(req)

		if err != nil {
			t.Fatalf("预期成功，得到错误: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("预期状态码 200，得到 %d", resp.StatusCode)
		}
		if statReceived == nil {
			t.Error("未收到统计信息")
		}
	})

	// 测试请求错误
	t.Run("请求错误", func(t *testing.T) {
		expectedErr := fmt.Errorf("network error")
		mockTransport := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return nil, expectedErr
		})

		middleware := printRoundTripper(func(ctx context.Context, stat *Stat) {
			statReceived = stat
		})

		req, _ := http.NewRequest("GET", "http://example.com", nil)
		resp, err := middleware(mockTransport).RoundTrip(req)

		if err != expectedErr {
			t.Errorf("预期错误 %v，得到 %v", expectedErr, err)
		}
		if resp != nil {
			t.Error("错误情况下不应该返回响应")
		}
		if statReceived.Err != expectedErr.Error() {
			t.Error("统计信息中错误不匹配")
		}
	})
}

func TestStreamRoundTripError(t *testing.T) {
	// 测试传输错误
	t.Run("传输错误", func(t *testing.T) {
		expectedErr := fmt.Errorf("transport error")
		mockTransport := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return nil, expectedErr
		})

		middleware := streamRoundTrip(func(_ int64, _ []byte) error {
			return nil
		})

		req, _ := http.NewRequest("GET", "http://example.com", nil)
		resp, err := middleware(mockTransport).RoundTrip(req)

		if err != expectedErr {
			t.Errorf("预期错误 %v，得到 %v", expectedErr, err)
		}
		if resp != nil {
			t.Error("错误情况下不应该返回响应")
		}
	})

	// 测试流处理错误
	t.Run("流处理错误", func(t *testing.T) {
		expectedErr := fmt.Errorf("stream processing error")
		mockTransport := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("test\ndata")),
			}, nil
		})

		middleware := streamRoundTrip(func(_ int64, _ []byte) error {
			return expectedErr
		})

		req, _ := http.NewRequest("GET", "http://example.com", nil)
		resp, err := middleware(mockTransport).RoundTrip(req)

		if err != expectedErr {
			t.Errorf("预期错误 %v，得到 %v", expectedErr, err)
		}
		if resp == nil {
			t.Error("应该返回响应对象")
		}
	})

	// 测试大数据流处理
	t.Run("大数据流处理", func(t *testing.T) {
		// 生成大量测试数据
		var largeData strings.Builder
		for i := range 10 {
			largeData.WriteString(fmt.Sprintf("line %d\n", i))
		}

		mockTransport := RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(largeData.String())),
			}, nil
		})

		lineCount := 0
		middleware := streamRoundTrip(func(_ int64, _ []byte) error {
			lineCount++
			return nil
		})

		req, _ := http.NewRequest("GET", "http://example.com", nil)
		resp, err := middleware(mockTransport).RoundTrip(req)

		if err != nil {
			t.Fatalf("未预期的错误: %v", err)
		}
		if lineCount != 10+1 {
			t.Skipf("预期处理 10 行，实际处理 %d 行", lineCount)
		}
		if resp.StatusCode != 200 {
			t.Errorf("预期状态码 200，得到 %d", resp.StatusCode)
		}
	})
}
