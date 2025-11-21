package requests

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// mockResponseWriter implements http.ResponseWriter for testing
// 更新 mockResponseWriter 以支持错误测试
type mockResponseWriter struct {
	headers    http.Header
	statuscode int
	body       bytes.Buffer
	writeError error // 添加这个字段
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	if m.writeError != nil {
		return 0, m.writeError
	}
	return m.body.Write(b)
}

func newMockResponseWriter() *mockResponseWriter {
	return &mockResponseWriter{headers: make(http.Header)}
}

func (m *mockResponseWriter) Header() http.Header  { return m.headers }
func (m *mockResponseWriter) WriteHeader(code int) { m.statuscode = code }

// TestResponseWriterBasic tests basic functionality of ResponseWriter
func TestResponseWriterBasic(t *testing.T) {
	mock := newMockResponseWriter()
	w := NewResponseWriter(mock)

	// Test WriteHeader
	w.WriteHeader(http.StatusCreated)
	if w.StatusCode != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, w.StatusCode)
	}

	// Test multiple WriteHeader calls
	w.WriteHeader(http.StatusOK)
	if w.StatusCode != http.StatusCreated {
		t.Error("WriteHeader should not change status code on subsequent calls")
	}

	// Test Write
	content := []byte("test content")
	n, err := w.Write(content)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(content) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(content), n)
	}
	if !bytes.Equal(w.Content.Bytes(), content) {
		t.Error("Written content does not match")
	}
}

// TestResponseWriterIntegration tests the ResponseWriter in a real HTTP server context
func TestResponseWriterIntegration(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := NewResponseWriter(w)
		rw.WriteHeader(http.StatusAccepted)
		rw.Write([]byte("hello world"))
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("Expected status %d, got %d", http.StatusAccepted, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if string(body) != "hello world" {
		t.Errorf("Expected body 'hello world', got '%s'", string(body))
	}
}

// TestResponseWriterConcurrency tests concurrent writes to ResponseWriter
func TestResponseWriterConcurrency(t *testing.T) {
	mock := newMockResponseWriter()
	w := NewResponseWriter(mock)

	var wg sync.WaitGroup
	workers := 10
	iterations := 100

	wg.Add(workers)
	for i := range workers {
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				content := fmt.Appendf(nil, "worker%d-%d", id, j)
				_, err := w.Write(content)
				if err != nil {
					t.Errorf("Concurrent write failed: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify that all writes were recorded
	if w.Content.Len() == 0 {
		t.Error("No content was written in concurrent test")
	}
}

// TestResponseWriterHijack tests the hijack functionality
func TestResponseWriterHijack(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hijackable := NewResponseWriter(w)
		conn, bufrw, err := hijackable.Hijack()
		if err != nil {
			t.Errorf("Hijack failed: %v", err)
			return
		}
		defer conn.Close()

		// Write a custom response
		bufrw.WriteString("HTTP/1.1 200 OK\r\n\r\nHijacked Response")
		bufrw.Flush()
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if string(body) != "Hijacked Response" {
		t.Errorf("Expected 'Hijacked Response', got '%s'", string(body))
	}
}

// BenchmarkResponseWriterWrite benchmarks the Write method
func BenchmarkResponseWriterWrite(b *testing.B) {
	mock := newMockResponseWriter()
	w := NewResponseWriter(mock)
	content := []byte("benchmark content")

	b.ResetTimer()
	for range b.N {
		w.Write(content)
	}
}

// BenchmarkResponseWriterConcurrentWrite benchmarks concurrent writes
func BenchmarkResponseWriterConcurrentWrite(b *testing.B) {
	mock := newMockResponseWriter()
	w := NewResponseWriter(mock)
	content := []byte("concurrent benchmark content")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w.Write(content)
		}
	})
}

// TestResponseWriterFlush tests the flush functionality
func TestResponseWriterFlush(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := NewResponseWriter(w)
		flusher.Write([]byte("chunk1"))
		flusher.Flush()
		time.Sleep(10 * time.Millisecond)
		flusher.Write([]byte("chunk2"))
		flusher.Flush()
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	chunk1, err := reader.ReadString('1')
	if err != nil || chunk1 != "chunk1" {
		t.Errorf("Expected chunk1, got %s, err: %v", chunk1, err)
	}

	chunk2, err := reader.ReadString('2')
	if err != nil || chunk2 != "chunk2" {
		t.Errorf("Expected chunk2, got %s, err: %v", chunk2, err)
	}
}

// TestResponseWriterRead 测试 Read 方法
func TestResponseWriterRead(t *testing.T) {
	mock := newMockResponseWriter()
	w := NewResponseWriter(mock)

	// 写入测试数据
	testData := []byte("test data for reading")
	_, err := w.Write(testData)
	if err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}

	// 测试读取
	buf := make([]byte, len(testData))
	n, err := w.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Expected to read %d bytes, got %d", len(testData), n)
	}
	if string(buf) != string(testData) {
		t.Errorf("Expected to read '%s', got '%s'", string(testData), string(buf))
	}

	// 测试读取完后的EOF
	n, err = w.Read(buf)
	if err != io.EOF {
		t.Errorf("Expected EOF after reading all data, got n=%d, err=%v", n, err)
	}
}

// TestResponseWriterPush 测试 HTTP/2 Push 功能
func TestResponseWriterPush(t *testing.T) {
	// 创建支持 HTTP/2 的测试服务器
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := NewResponseWriter(w)
		err := rw.Push("/style.css", &http.PushOptions{
			Method: "GET",
			Header: http.Header{
				"Content-Type": []string{"text/css"},
			},
		})
		if err != nil && err != http.ErrNotSupported {
			t.Errorf("Push failed: %v", err)
		}
		rw.Write([]byte("main content"))
	})

	server := httptest.NewUnstartedServer(handler)
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()

	// 发起请求
	client := server.Client()
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}
	if string(body) != "main content" {
		t.Errorf("Expected 'main content', got '%s'", string(body))
	}
}

// TestResponseWriterWriteError 测试 Write 方法的错误处理
func TestResponseWriterWriteError(t *testing.T) {
	// 创建一个会返回错误的 mock
	errMock := &mockResponseWriter{
		headers:    make(http.Header),
		writeError: fmt.Errorf("write error"),
	}

	w := NewResponseWriter(errMock)

	// 测试写入错误
	n, err := w.Write([]byte("test"))
	if err == nil {
		t.Error("Expected write error, got nil")
	}
	if n != 0 {
		t.Errorf("Expected 0 bytes written on error, got %d", n)
	}
}
