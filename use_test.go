package requests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestServerSentEvents_Basic 测试 ServerSentEvents 的基本功能
func TestServerSentEvents_Basic(t *testing.T) {
	w := httptest.NewRecorder()
	sse := &ServerSentEvents{w: w}

	// 测试 WriteHeader
	sse.WriteHeader(http.StatusOK)
	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 得到 %d", http.StatusOK, w.Code)
	}

	// 测试 Header
	sse.Header().Set("Test", "value")
	if w.Header().Get("Test") != "value" {
		t.Error("Header 设置失败")
	}

	// 测试 Write
	data := []byte("test data")
	n, err := sse.Write(data)
	if err != nil {
		t.Errorf("Write 失败: %v", err)
	}
	if !strings.Contains(w.Body.String(), "data:test data\n") {
		t.Error("Write 输出格式错误")
	}
	if n != len("data:test data\n") {
		t.Error("Write 返回长度错误")
	}

	// 测试 Send
	_, err = sse.Send("event", []byte("test event"))
	if err != nil {
		t.Errorf("Send 失败: %v", err)
	}
	if !strings.Contains(w.Body.String(), "event:test event\n") {
		t.Error("Send 输出格式错误")
	}

	// 测试 End
	sse.End()
	if !strings.HasSuffix(w.Body.String(), "\n\n") {
		t.Error("End 没有正确添加结束标记")
	}
}

// TestServerSentEvents_Read 测试 Read 方法的所有分支
func TestServerSentEvents_Read(t *testing.T) {
	sse := &ServerSentEvents{}
	tests := []struct {
		name     string
		input    []byte
		wantData []byte
		wantErr  bool
	}{
		{
			name:     "空行",
			input:    []byte("\n"),
			wantData: nil,
			wantErr:  false,
		},
		{
			name:     "注释行",
			input:    []byte(": comment\n"),
			wantData: nil,
			wantErr:  false,
		},
		{
			name:     "事件声明",
			input:    []byte("event:message\n"),
			wantData: nil,
			wantErr:  false,
		},
		{
			name:     "数据行",
			input:    []byte("data:test data\n"),
			wantData: []byte("test data"),
			wantErr:  false,
		},
		{
			name:     "未知事件",
			input:    []byte("unknown:data\n"),
			wantData: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := sse.Read(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Read() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !bytes.Equal(data, tt.wantData) {
				t.Errorf("Read() = %v, want %v", data, tt.wantData)
			}
		})
	}
}

// TestSSEMiddleware 测试 SSE 中间件
func TestSSEMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test message"))
	})

	server := httptest.NewServer(SSE()(handler))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 验证响应头
	expectedHeaders := map[string]string{
		"Content-Type":                 "text/event-stream",
		"Cache-Control":                "no-cache",
		"Connection":                   "keep-alive",
		"Access-Control-Allow-Headers": "Content-Type",
		"Access-Control-Allow-Origin":  "*",
	}

	for k, v := range expectedHeaders {
		if resp.Header.Get(k) != v {
			t.Errorf("期望 header %s=%s, 得到 %s", k, v, resp.Header.Get(k))
		}
	}

	// 验证响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应失败: %v", err)
	}
	if !strings.Contains(string(body), "data:test message") {
		t.Error("响应内容格式错误")
	}
}

// TestCORSMiddleware 测试 CORS 中间件
func TestCORSMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	server := httptest.NewServer(CORS()(handler))
	defer server.Close()

	// 测试 OPTIONS 请求
	req, _ := http.NewRequest(http.MethodOptions, server.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS 请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("OPTIONS 请求期望状态码 %d, 得到 %d", http.StatusNoContent, resp.StatusCode)
	}

	// 测试正常请求
	resp, err = http.Get(server.URL)
	if err != nil {
		t.Fatalf("GET 请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 验证 CORS 头
	expectedHeaders := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, OPTIONS",
		"Access-Control-Allow-Headers": "Content-Type, Authorization",
	}

	for k, v := range expectedHeaders {
		if resp.Header.Get(k) != v {
			t.Errorf("期望 header %s=%s, 得到 %s", k, v, resp.Header.Get(k))
		}
	}
}

// TestPrintHandler 测试打印处理器
func TestPrintHandler(t *testing.T) {
	var statReceived *Stat
	printFunc := func(ctx context.Context, stat *Stat) {
		statReceived = stat
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	server := httptest.NewServer(printHandler(printFunc)(handler))
	defer server.Close()

	// 发送带 body 的 POST 请求
	resp, err := http.Post(server.URL, "application/json", strings.NewReader(`{"test":"data"}`))
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 验证统计信息
	if statReceived == nil {
		t.Fatal("未收到统计信息")
	}
	if statReceived.Response.StatusCode != http.StatusOK {
		t.Errorf("统计信息状态码错误: 期望 %d, 得到 %d", http.StatusOK, statReceived.Response.StatusCode)
	}
	if statReceived.Cost < 0 {
		t.Errorf("统计信息处理时间异常: cost=%d", statReceived.Cost)
	}
}

func requestIdMiddle() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 从请求头中获取 request-id
			requestId := GenId(r.Header.Get("request-id"))
			// 将 request-id 添加到响应头
			r.Header.Set("request-id", requestId)
			w.Header().Set("request-id", requestId)
			// 调用下一个处理器
			// 定义请求ID的上下文键类型
			type requestIDKey struct{}

			// 使用自定义类型作为上下文键
			r2 := r.WithContext(context.WithValue(r.Context(), requestIDKey{}, requestId))
			next.ServeHTTP(w, r2)
		})
	}
}

func Test_SSE(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := NewServeMux(Logf(LogS), Use(requestIdMiddle()))
	r.Route("/123", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("hello world"))
	}, Method("PUT"))
	r.Route("/sse", func(w http.ResponseWriter, r *http.Request) {

		for i := range 3 {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(1 * time.Second):
				w.Write(fmt.Appendf(nil, `{"a":"12345\n", "b": %d}`, i))
			}
		}
	}, Use(SSE()), Method("DELETE"))
	s := NewServer(ctx, r, URL("http://0.0.0.0:1234"))
	go s.ListenAndServe()
	time.Sleep(1 * time.Second)
	c := New(Logf(LogS))
	_, err := c.DoRequest(ctx, URL("http://0.0.0.0:1234/sse"),
		Stream(func(i int64, b []byte) error {

			t.Logf("i=%d, b=%s", i, b)
			return nil

		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.DoRequest(ctx, URL("http://0.0.0.0:1234/123"), Body(`{"a":"b"}`))
	if err != nil {
		t.Fatal(err)
	}

	cancel()

	time.Sleep(1 * time.Second)
}

func SSERound(i int64, b []byte, f func([]byte) error) error {
	name, value, _ := bytes.Cut(bytes.TrimRight(b, "\n"), []byte(":"))
	switch string(name) {
	case "data":
		return f(value)
	default:
		return nil
	}
}

func Test_UseStep(t *testing.T) {
	var ss []string
	var use = func(stage, step string) func(next http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ss = append(ss, fmt.Sprintf("%s-%s-start", stage, step))
				t.Logf("use: %s-%s-start", stage, step)
				next.ServeHTTP(w, r)
				ss = append(ss, fmt.Sprintf("%s-%s-end", stage, step))
				t.Logf("use: %s-%s-end", stage, step)
			})
		}
	}

	mux := NewServeMux(
		Use(requestIdMiddle()),
		Logf(func(ctx context.Context, stat *Stat) {
			t.Logf("mux: Logf: %v", ctx.Value("request-id"))
		}),
		Use(use("mux", "1")),
		Use(use("mux", "2")),
		Use(use("mux", "3")),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mux.Route("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello world"))
	}, Use(use("route", "1"), use("route", "2"), use("route", "3")))
	server := NewServer(ctx, mux, URL("http://0.0.0.0:9090"), Use(use("server", "1"), use("server", "2"), use("server", "3")))
	go server.ListenAndServe()

	Get("http://127.0.0.1:9090/")
	time.Sleep(1 * time.Second)
}
