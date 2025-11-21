package requests

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestParseBody_NilBody(t *testing.T) {
	// 测试 nil body
	buf, err := ParseBody(nil)
	if err != nil {
		t.Errorf("ParseBody(nil) 返回错误: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("ParseBody(nil) 应返回空缓冲区，实际长度: %d", buf.Len())
	}
}

func TestParseBody_NoBody(t *testing.T) {
	// 测试 http.NoBody
	buf, err := ParseBody(http.NoBody)
	if err != nil {
		t.Errorf("ParseBody(http.NoBody) 返回错误: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("ParseBody(http.NoBody) 应返回空缓冲区，实际长度: %d", buf.Len())
	}
}

func TestParseBody_WithContent(t *testing.T) {
	// 测试有内容的 body
	content := "test content"
	body := io.NopCloser(strings.NewReader(content))

	buf, err := ParseBody(body)
	if err != nil {
		t.Errorf("ParseBody(body) 返回错误: %v", err)
	}
	if buf.String() != content {
		t.Errorf("ParseBody(body) 应返回 %q，实际返回: %q", content, buf.String())
	}
}

func TestCopyBody_NilBody(t *testing.T) {
	// 测试 nil body
	buf, body, err := CopyBody(nil)
	if err != nil {
		t.Errorf("CopyBody(nil) 返回错误: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("CopyBody(nil) 应返回空缓冲区，实际长度: %d", buf.Len())
	}

	// 验证返回的 body 是否可读
	content, err := io.ReadAll(body)
	if err != nil {
		t.Errorf("读取 CopyBody(nil) 返回的 body 失败: %v", err)
	}
	if len(content) != 0 {
		t.Errorf("CopyBody(nil) 返回的 body 应为空，实际长度: %d", len(content))
	}
}

func TestCopyBody_WithContent(t *testing.T) {
	// 测试有内容的 body
	content := "test content for copy"
	originalBody := io.NopCloser(strings.NewReader(content))

	buf, newBody, err := CopyBody(originalBody)
	if err != nil {
		t.Errorf("CopyBody(body) 返回错误: %v", err)
	}
	if buf.String() != content {
		t.Errorf("CopyBody(body) 返回的缓冲区应包含 %q，实际为: %q", content, buf.String())
	}

	// 验证返回的 body 是否包含相同内容
	newContent, err := io.ReadAll(newBody)
	if err != nil {
		t.Errorf("读取 CopyBody(body) 返回的 body 失败: %v", err)
	}
	if string(newContent) != content {
		t.Errorf("CopyBody(body) 返回的 body 应包含 %q，实际为: %q", content, string(newContent))
	}
}

func TestLogS(t *testing.T) {
	// 创建一个基本的 Stat 对象
	stat := &Stat{
		RequestId: "test-request-id",
		StartAt:   "2023-05-01 12:00:00.000",
		Cost:      100,
	}
	stat.Request.Method = "GET"
	stat.Request.URL = "http://example.com/test"
	stat.Request.Body = map[string]interface{}{"key": "value"}
	stat.Response.StatusCode = 200
	stat.Response.ContentLength = 1024
	stat.Response.Body = "response body"

	// 测试 LogS 函数 - 这里我们只是确保它不会崩溃
	// 由于它使用 log 输出，我们不捕获输出进行验证
	LogS(context.Background(), stat)

	// 测试 Response.URL 为空的情况
	emptyStat := &Stat{
		RequestId: "empty-url-test",
		StartAt:   "2023-05-01 12:00:00.000",
		Cost:      50,
	}
	emptyStat.Request.Method = "GET"
	emptyStat.Request.URL = "http://example.com/empty"
	emptyStat.Response.StatusCode = 200

	// 确保不会崩溃
	LogS(context.Background(), emptyStat)

	// 测试 Request.Body 无法序列化为 JSON 的情况
	badStat := &Stat{
		RequestId: "bad-body-test",
		StartAt:   "2023-05-01 12:00:00.000",
		Cost:      75,
	}
	badStat.Request.Method = "POST"
	badStat.Request.URL = "http://example.com/bad"
	badStat.Response.URL = "http://example.com"
	badStat.Response.StatusCode = 400

	// 创建一个包含循环引用的 body，这会导致 JSON 序列化失败
	type CircularRef struct {
		Name string
		Self *CircularRef
	}
	circular := &CircularRef{Name: "circular"}
	circular.Self = circular
	badStat.Request.Body = circular

	// 确保不会崩溃
	LogS(context.Background(), badStat)
}

// 测试 ParseBody 处理读取错误的情况
func TestParseBody_ReadError(t *testing.T) {
	// 创建一个会返回错误的 ReadCloser
	errReader := &errorReadCloser{err: io.ErrUnexpectedEOF}

	_, err := ParseBody(errReader)
	if err != io.ErrUnexpectedEOF {
		t.Errorf("ParseBody 应返回读取错误，实际返回: %v", err)
	}
}

// 实现一个总是返回错误的 ReadCloser
type errorReadCloser struct {
	err error
}

func (e *errorReadCloser) Read(p []byte) (n int, err error) {
	return 0, e.err
}

func (e *errorReadCloser) Close() error {
	return nil
}

// 测试 CopyBody 处理读取错误的情况
func TestCopyBody_ReadError(t *testing.T) {
	// 创建一个会返回错误的 ReadCloser
	errReader := &errorReadCloser{err: io.ErrUnexpectedEOF}

	_, _, err := CopyBody(errReader)
	if err != io.ErrUnexpectedEOF {
		t.Errorf("CopyBody 应返回读取错误，实际返回: %v", err)
	}
}

// 基准测试 ParseBody
func BenchmarkParseBody(b *testing.B) {
	content := strings.Repeat("benchmark content for ParseBody", 100)

	b.ResetTimer()
	for range b.N {
		body := io.NopCloser(strings.NewReader(content))
		_, _ = ParseBody(body)
	}
}

// 基准测试 CopyBody
func BenchmarkCopyBody(b *testing.B) {
	content := strings.Repeat("benchmark content for CopyBody", 100)

	b.ResetTimer()
	for range b.N {
		body := io.NopCloser(strings.NewReader(content))
		_, _, _ = CopyBody(body)
	}
}
