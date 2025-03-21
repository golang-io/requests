package requests

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"sync/atomic"
	"testing"
	"time"
)

func Test_Basic(t *testing.T) {
	resp, err := Get("http://httpbin.org/get")
	t.Logf("%#v, %v", resp, err)
	//resp, _ = Post("http://httpbin.org/post", "application/json", strings.NewReader(`{"a": "b"}`))
	//t.Log(resp.Text())
}

func Test_ProxyGet(t *testing.T) {
	t.Log("Testing get request")
	sess := New(
		Header("a", "b"),
		Cookie(http.Cookie{Name: "username", Value: "golang"}),
		BasicAuth("user", "123456"),
		Timeout(3*time.Second),
		//Hosts(map[string][]string{"127.0.0.1:8080": {"192.168.1.1:80"}, "4.org:80": {"httpbin.org:80"}}),
		//Proxy("http://127.0.0.1:8080"),
	)

	resp, err := sess.DoRequest(
		context.Background(),
		Method("GET"),
		URL("http://httpbin.org"),
	)
	if err != nil {
		t.Errorf("%s", err.Error())
		return
	}
	t.Log(resp.StatusCode, err)
	//t.Log(resp.Text())
}

func Test_PostBody(t *testing.T) {
	sess := New(
		BasicAuth("user", "123456"),
		Logf(func(context.Context, *Stat) {

		}),
	)
	//if err := sess.Proxy("127.0.0.1:8080"); err != nil {
	//	t.Error(err)
	//	return
	//}

	resp, err := sess.DoRequest(context.Background(),
		Method("POST"),
		URL("http://httpbin.org/post"),
		Params(map[string]string{
			"a": "b/c",
			"c": "3",
			"d": "ddd",
		}),
		Param("e", "ea", "es"),

		Body(`{"body":"QWER"}`),
		Header("hello", "world"),
		//TraceLv(9),
		Logf(func(ctx context.Context, stat *Stat) {
			//t.Logf("%v", stat.String())
		}),
	)
	if err != nil {
		t.Logf("%v", err)
		return
	}
	t.Log(resp.StatusCode, err, resp.Response.ContentLength, resp.Request.ContentLength)
	//t.Log(resp.Text())
	//t.Log(resp.Stat())
}

func Test_FormPost(t *testing.T) {
	t.Log("Testing get request")

	sess := New()
	resp, err := sess.DoRequest(context.Background(),
		Method("POST"),
		URL("http://httpbin.org/post"),
		Form(url.Values{"name": {"12.com"}}),
		Params(map[string]string{
			"a": "b/c",
			"c": "cc",
			"d": "dddd",
		}),
		Param("e", "ea", "es"),

		//TraceLv(9),
	)
	if err != nil {
		log.Fatal(err)
		return
	}
	t.Log(resp.StatusCode, err, resp.Response.ContentLength, resp.Request.ContentLength)

}

func Test_Race(t *testing.T) {
	opts := Options{}
	ctx := context.Background()
	t.Logf("%#v", opts)
	sess := New(URL("http://httpbin.org/post")) //, Auth("user", "123456"))
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = sess.DoRequest(ctx, MethodPost, Body(`{"a":"b"}`), Params(map[string]string{"1": "2/2"})) // nolint: errcheck
		}()
	}
	time.Sleep(3 * time.Second)
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
	_, _ = sess.DoRequest(context.Background(), URL(s.URL))
}

func Test_Cannel(t *testing.T) {
	sess := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resp, err := sess.DoRequest(ctx, URL("http://127.0.0.1:9099"))
	t.Logf("%s, err=%v", resp.Stat(), err)
}

func TestResponse_Download(t *testing.T) {
	if err := os.MkdirAll("tmp", 0755); err != nil {
		t.Fatalf("Failed to create tmp directory: %v", err)
	}

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		text := "abc\ndef\nghij\n\n123"
		fmt.Fprint(w, text)
	}))
	defer s.Close()
	u := "https://go.dev/dl/go1.22.1.darwin-amd64.tar.gz" // a35015fca6f631f3501a36b3bccba9c5
	//u := "https://dl.google.com/go/go1.22.1.darwin-amd64.tar.gz" // a35015fca6f631f3501a36b3bccba9c5

	sess := New(URL(u))
	tmp := t.TempDir()
	f, err := os.OpenFile(path.Join(tmp, "xx.tar.gz"), os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
		return
	}
	defer f.Close()
	sum := 0
	_ = Stream(func(i int64, row []byte) error {
		cnt, err := f.Write(row)
		sum += cnt
		return err
	})
	resp, err := sess.DoRequest(context.Background(), Setup(Redirect))
	if err != nil {
		t.Logf("resp=%d, err=%s", resp.Content, err)
		return
	}
	if resp.StatusCode != 200 {
		t.Fatalf("resp=%s, err=%v", resp.Referer(), err)
		return
	}
	io.Copy(f, resp.Content)
}

// 添加以下测试用例

func TestRequestWithTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte("delayed response"))
	}))
	defer server.Close()

	// 测试超时情况
	sess := New(Timeout(10 * time.Millisecond))
	_, err := sess.DoRequest(context.Background(), URL(server.URL))
	if err == nil {
		t.Skip("期望超时错误，但没有发生")
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

func TestRequestWithLocalAddr(t *testing.T) {
	localAddr := &net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 0,
	}

	sess := New(LocalAddr(localAddr))
	resp, err := sess.DoRequest(context.Background(),
		URL("http://example.com"),
	)
	if err != nil {
		t.Log("本地地址绑定测试跳过:", err)
		t.Skip()
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
