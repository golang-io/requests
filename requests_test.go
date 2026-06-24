package requests

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var s *Server

func TestMain(m *testing.M) {
	mux := NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello world"))
	})
	s = NewServer(context.Background(), mux, URL("http://127.0.0.1:65534"))

	go s.ListenAndServe()
	defer s.Shutdown(context.Background())
	os.Exit(m.Run())
}

func TestDeRequest_Trace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sse := &ServerSentEvents{w: w}
		defer sse.End()
		for range 10 {
			time.Sleep(1 * time.Second)
			sse.Write([]byte("test."))
		}
	}))
	defer server.Close()
	sess := New(Timeout(20 * time.Second))
	resp, err := sess.DoRequest(context.Background(), URL(server.URL), Method("GET"), Trace(), Logf(LogS), Stream(func(i int64, row []byte) error {
		fmt.Fprintf(os.Stderr, "streamOutput: %s", row)
		return nil
	}))
	t.Logf("resp=%v, err=%v", resp.Content.String(), err)
}

// Test_ProxyGet 测试带自定义 Header、Cookie、BasicAuth 的 GET 请求。
// 使用 httptest.Server 替代外部 httpbin.org，离线可运行。
func Test_ProxyGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证 Header
		if r.Header.Get("a") != "b" {
			t.Errorf("期望 Header a=b，实际 %q", r.Header.Get("a"))
		}
		// 验证 Cookie
		cookie, err := r.Cookie("username")
		if err != nil || cookie.Value != "golang" {
			t.Errorf("期望 Cookie username=golang，实际 %v", err)
		}
		// 验证 BasicAuth
		user, pass, ok := r.BasicAuth()
		if !ok || user != "user" || pass != "123456" {
			t.Errorf("BasicAuth 验证失败: user=%s pass=%s ok=%v", user, pass, ok)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	sess := New(
		Header("a", "b"),
		Cookie(http.Cookie{Name: "username", Value: "golang"}),
		BasicAuth("user", "123456"),
	)
	resp, err := sess.DoRequest(context.Background(), Method("GET"), URL(server.URL))
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 200，实际 %d", resp.StatusCode)
	}
}

// Test_PostBody 测试带 query 参数、JSON body、自定义 Header 的 POST 请求。
// 使用 httptest.Server 替代外部 httpbin.org，离线可运行。
func Test_PostBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证方法
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST，实际 %s", r.Method)
		}
		// 验证 query 参数
		q := r.URL.Query()
		if q.Get("a") != "b/c" {
			t.Errorf("期望 a=b/c，实际 %q", q.Get("a"))
		}
		// 验证自定义 Header
		if r.Header.Get("hello") != "world" {
			t.Errorf("期望 header hello=world，实际 %q", r.Header.Get("hello"))
		}
		// 验证 body
		body, _ := io.ReadAll(r.Body)
		var m map[string]string
		if err := json.Unmarshal(body, &m); err != nil || m["body"] != "QWER" {
			t.Errorf("body 不符合预期: %s", body)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	sess := New(BasicAuth("user", "123456"))
	resp, err := sess.DoRequest(context.Background(),
		Method("POST"),
		URL(server.URL),
		Params(map[string]string{"a": "b/c", "c": "3", "d": "ddd"}),
		Param("e", "ea", "es"),
		Body(`{"body":"QWER"}`),
		Header("hello", "world"),
		Logf(func(ctx context.Context, stat *Stat) {
			t.Logf("%v", stat.String())
		}),
	)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	t.Log(resp.StatusCode, resp.Content.String())
}

// Test_FormPost 测试 form 表单 POST，含 query 参数。
// 使用 httptest.Server 替代外部 httpbin.org，离线可运行。
func Test_FormPost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("解析表单失败: %v", err)
		}
		if r.FormValue("name") != "12.com" {
			t.Errorf("期望 name=12.com，实际 %q", r.FormValue("name"))
		}
		if r.URL.Query().Get("a") != "b/c" {
			t.Errorf("期望 query a=b/c，实际 %q", r.URL.Query().Get("a"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("form ok"))
	}))
	defer server.Close()

	sess := New()
	resp, err := sess.DoRequest(context.Background(),
		Method("POST"),
		URL(server.URL),
		Form(url.Values{"name": {"12.com"}}),
		Params(map[string]string{"a": "b/c", "c": "cc", "d": "dddd"}),
		Param("e", "ea", "es"),
	)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	t.Log(resp.StatusCode, resp.Content.String())
}

// Test_DoRequestRace 测试并发 DoRequest 无数据竞争。
// 使用 httptest.Server 替代外部 httpbin.org，离线可运行。
func Test_DoRequestRace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(w, r.Body)
	}))
	defer server.Close()

	ctx := context.Background()
	sess := New(URL(server.URL))
	done := make(chan struct{})
	for range 10 {
		go func() {
			defer func() { done <- struct{}{} }()
			_, _ = sess.DoRequest(ctx, MethodPost, Body(`{"a":"b"}`), Params(map[string]string{"1": "2/2"}))
		}()
	}
	for range 10 {
		<-done
	}
}

func Test_Retry(t *testing.T) {
	var reqCount int32 = 0

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqNo := atomic.AddInt32(&reqCount, 1)
		if reqNo%3 == 0 {
			w.WriteHeader(404)
		} else {
			w.WriteHeader(200)
		}
		_, _ = w.Write([]byte(fmt.Sprintf("response: %d", reqNo)))
	}))
	defer s.Close()

	sess := New()
	_, _ = sess.DoRequest(context.Background(), URL(s.URL), Trace())
}

func Test_Cannel(t *testing.T) {
	sess := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resp, err := sess.DoRequest(ctx, URL("http://127.0.0.1:9099"))
	t.Logf("%s, err=%v", resp.Stat(), err)
}

// TestDownload 测试流式下载并写入文件。
// 使用 httptest.Server 提供虚拟文件内容，替代外部 go.dev 下载链接，离线可运行。
func TestDownload(t *testing.T) {
	fileContent := "abc\ndef\nghij\n\n123"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		fmt.Fprint(w, fileContent)
	}))
	defer server.Close()

	tmp := t.TempDir()
	f, err := os.OpenFile(path.Join(tmp, "download.bin"), os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer f.Close()

	sess := New(URL(server.URL))
	resp, err := sess.DoRequest(context.Background(), Trace())
	if err != nil {
		t.Fatalf("下载失败: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("期望状态码 200，实际 %d", resp.StatusCode)
	}
	if _, err := io.Copy(f, resp.Content); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}

	// 验证写入内容一致
	written, _ := os.ReadFile(path.Join(tmp, "download.bin"))
	if string(written) != fileContent {
		t.Errorf("下载内容不匹配\n期望: %q\n实际: %q", fileContent, written)
	}
}

func TestRequestWithTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte("delayed response"))
	}))
	defer server.Close()

	setup := func(name string) func(next http.RoundTripper) http.RoundTripper {
		return func(next http.RoundTripper) http.RoundTripper {
			return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
				t.Logf("timeout %s test prev", name)
				defer t.Logf("timeout %s test next", name)
				return next.RoundTrip(r)
			})
		}
	}

	// 测试超时情况
	sess := New(Timeout(10*time.Millisecond), Logf(LogS), Setup(setup("session0"), setup("session1")))
	_, err := sess.DoRequest(context.Background(), URL(server.URL), Setup(setup("request0-0"), setup("request0-1")))
	t.Logf("timeout err=%v", err)
	if err == nil {
		t.Error("期望发生超时错误，但没有")
	}
	if !strings.Contains(err.Error(), "Client.Timeout exceeded") {
		t.Error("发生错误，但不是超时")
	}

	// 测试非超时情况
	sess = New(Timeout(200 * time.Millisecond))
	resp, err := sess.DoRequest(context.Background(), URL(server.URL))
	if err != nil {
		t.Errorf("不期望发生错误: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", resp.StatusCode)
	}
}

func TestRequestWithGzip(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Encoding") != "gzip" {
			t.Error("请求未使用 gzip 编码")
		}
		// 验证 body 可以正常解压
		gr, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Errorf("创建 gzip reader 失败: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer gr.Close()
		body, _ := io.ReadAll(gr)
		if !strings.Contains(string(body), "data") {
			t.Errorf("解压后 body 内容不符合预期: %s", body)
		}
		w.Write([]byte("response"))
	}))
	defer server.Close()

	sess := New()
	resp, err := sess.DoRequest(context.Background(),
		URL(server.URL),
		Gzip(`{"test":"data"}`),
	)
	if err != nil {
		t.Errorf("不期望发生错误: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", resp.StatusCode)
	}
}

func TestRequestWithProxy(t *testing.T) {
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("proxy response"))
	}))
	defer proxyServer.Close()

	sess := New(Proxy(proxyServer.URL))
	resp, err := sess.DoRequest(context.Background(),
		URL("http://example.com"),
	)
	if err != nil {
		t.Errorf("不期望发生错误: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", resp.StatusCode)
	}
}

// TestRequestWithLocalAddr 测试绑定本地地址发起请求（目标为 httptest.Server，离线可运行）。
func TestRequestWithLocalAddr(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("local addr ok"))
	}))
	defer server.Close()

	localAddr := &net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 0,
	}
	sess := New(LocalAddr(localAddr))
	resp, err := sess.DoRequest(context.Background(), URL(server.URL))
	if err != nil {
		t.Fatalf("本地地址绑定请求失败: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", resp.StatusCode)
	}
}

func TestRequestWithVerify(t *testing.T) {
	// 创建自签名证书的 HTTPS 服务器
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("secure response"))
	}))
	defer server.Close()

	// 测试验证证书
	sess := New(Verify(true))
	_, err := sess.DoRequest(context.Background(), URL(server.URL))
	if err == nil {
		t.Error("期望自签名证书验证失败，但没有")
	}

	// 测试跳过证书验证
	sess = New(Verify(false))
	resp, err := sess.DoRequest(context.Background(), URL(server.URL))
	if err != nil {
		t.Errorf("不期望发生错误: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", resp.StatusCode)
	}
}

func TestRequestWithTrace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("response"))
	}))
	defer server.Close()

	var traced bool
	sess := New(
		Trace(1024),
		Logf(func(ctx context.Context, stat *Stat) {
			traced = true
		}),
	)

	resp, err := sess.DoRequest(context.Background(), URL(server.URL))
	if err != nil {
		t.Errorf("不期望发生错误: %v", err)
	}
	if !traced {
		t.Error("跟踪函数未被调用")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", resp.StatusCode)
	}
}
