package requests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestStat_String(t *testing.T) {
	stat := &Stat{
		RequestId: "test-request-id",
		StartAt:   "2023-05-01 12:00:00.000",
		Cost:      100,
	}
	stat.Request.Method = "GET"
	stat.Request.URL = "http://example.com/test"
	stat.Response.StatusCode = 200
	stat.Response.ContentLength = 1024

	jsonStr := stat.String()
	var parsedStat map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsedStat); err != nil {
		t.Errorf("无法解析 Stat.String() 的输出: %v", err)
	}

	if parsedStat["RequestId"] != "test-request-id" {
		t.Errorf("期望 RequestId 为 'test-request-id'，实际为 %v", parsedStat["RequestId"])
	}
}

func TestStat_Print(t *testing.T) {
	stat := &Stat{
		StartAt: "2023-05-01 12:00:00.000",
		Cost:    100,
	}
	stat.Request.Method = "GET"
	stat.Request.RemoteAddr = "192.168.1.1:8080"
	stat.Request.URL = "/api/v1/test"
	stat.Response.URL = "http://example.com"
	stat.Response.StatusCode = 200
	stat.Response.ContentLength = 1024

	printStr := stat.Print()
	expected := "2023-05-01 12:00:00.000 GET \"192.168.1.1:8080 -> http://example.com/api/v1/test\" - 200 1024B in 100ms"
	if printStr != expected {
		t.Errorf("期望输出为 '%s'，实际为 '%s'", expected, printStr)
	}
}

func TestResponseLoad(t *testing.T) {
	// 创建一个模拟的 HTTP 响应
	httpResp := &http.Response{
		StatusCode: 200,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Test":       []string{"test-value"},
		},
		Body: io.NopCloser(strings.NewReader(`{"message":"success"}`)),
	}

	// 创建一个模拟的请求
	req, _ := http.NewRequest("GET", "http://example.com/test?param=value", nil)
	req.Header.Set(RequestId, "test-request-id")
	req.Header.Set("User-Agent", "test-agent")

	// 创建响应对象
	resp := &Response{
		Response: httpResp,
		Request:  req,
		StartAt:  time.Now().Add(-100 * time.Millisecond), // 100ms 前
	}

	// 测试 responseLoad 函数
	stat := responseLoad(resp)

	// 验证基本字段
	if stat.RequestId != "test-request-id" {
		t.Errorf("期望 RequestId 为 'test-request-id'，实际为 %s", stat.RequestId)
	}

	if stat.Request.Method != "GET" {
		t.Errorf("期望 Method 为 'GET'，实际为 %s", stat.Request.Method)
	}

	if !strings.Contains(stat.Request.URL, "http://example.com/test?param=value") {
		t.Errorf("期望 URL 包含 'http://example.com/test?param=value'，实际为 %s", stat.Request.URL)
	}

	if stat.Response.StatusCode != 200 {
		t.Errorf("期望 StatusCode 为 200，实际为 %d", stat.Response.StatusCode)
	}

	if stat.Response.Header["Content-Type"] != "application/json" {
		t.Errorf("期望 Content-Type 为 'application/json'，实际为 %s", stat.Response.Header["Content-Type"])
	}

	// 验证响应体解析
	responseBody, ok := stat.Response.Body.(map[string]interface{})
	if !ok {
		t.Errorf("期望 Response.Body 为 map[string]interface{}，实际为 %T", stat.Response.Body)
	} else if responseBody["message"] != "success" {
		t.Errorf("期望 message 为 'success'，实际为 %v", responseBody["message"])
	}
}

func TestServeLoad(t *testing.T) {
	// 创建一个模拟的 HTTP 请求
	req, _ := http.NewRequest("POST", "/api/v1/test?param=value", strings.NewReader(`{"data":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "test-agent")
	req.RemoteAddr = "192.168.1.1:8080"

	// 创建一个模拟的响应写入器
	w := &ResponseWriter{
		StatusCode: 201,
		Content:    bytes.NewBufferString(`{"status":"created"}`),
	}

	// 创建请求体缓冲区
	buf := bytes.NewBufferString(`{"data":"test"}`)

	// 测试 serveLoad 函数
	start := time.Now().Add(-200 * time.Millisecond) // 200ms 前
	stat := serveLoad(w, req, start, buf)

	// 验证基本字段
	if stat.Request.Method != "POST" {
		t.Errorf("期望 Method 为 'POST'，实际为 %s", stat.Request.Method)
	}

	if stat.Request.RemoteAddr != "192.168.1.1:8080" {
		t.Errorf("期望 RemoteAddr 为 '192.168.1.1:8080'，实际为 %s", stat.Request.RemoteAddr)
	}

	if !strings.Contains(stat.Request.URL, "/api/v1/test?param=value") {
		t.Errorf("期望 URL 包含 '/api/v1/test?param=value'，实际为 %s", stat.Request.URL)
	}

	if stat.Response.StatusCode != 201 {
		t.Errorf("期望 StatusCode 为 201，实际为 %d", stat.Response.StatusCode)
	}

	if stat.Response.ContentLength != int64(w.Content.Len()) { // `{"status":"created"}` 的长度
		t.Errorf("期望 ContentLength 为 %d，实际为 %d", int64(w.Content.Len()), stat.Response.ContentLength)
	}

	// 验证请求体解析
	requestBody, ok := stat.Request.Body.(map[string]interface{})
	if !ok {
		t.Errorf("期望 Request.Body 为 map[string]interface{}，实际为 %T", stat.Request.Body)
	} else if requestBody["data"] != "test" {
		t.Errorf("期望 data 为 'test'，实际为 %v", requestBody["data"])
	}

	// 验证响应体
	if stat.Response.Body != `{"status":"created"}` {
		t.Errorf("期望 Response.Body 为 '{\"status\":\"created\"}'，实际为 %v", stat.Response.Body)
	}
}

func TestStat_WithError(t *testing.T) {
	// 测试带有错误的情况
	resp := &Response{
		Err:     fmt.Errorf("测试错误"),
		StartAt: time.Now().Add(-50 * time.Millisecond),
	}

	stat := responseLoad(resp)
	if stat.Err != "测试错误" {
		t.Errorf("期望错误信息为 '测试错误'，实际为 '%s'", stat.Err)
	}
}
