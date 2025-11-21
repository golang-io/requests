package requests

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func Test_Setup(t *testing.T) {
	var setups []string
	var setup = func(stage, step string) func(next http.RoundTripper) http.RoundTripper {
		return func(next http.RoundTripper) http.RoundTripper {
			return RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				setups = append(setups, strings.Join([]string{stage, step, "start"}, "-"))
				resp, err := next.RoundTrip(req)
				setups = append(setups, strings.Join([]string{stage, step, "end"}, "-"))
				return resp, err
			})
		}
	}

	var wants = []string{
		"session-step1-start", "session-step2-start", "request-step1-start", "request-step2-start",
		"request-step2-end", "request-step1-end", "session-step2-end", "session-step1-end",
	}

	var ss = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, r.Body)
	}))
	sess := New(Setup(setup("session", "step1"), setup("session", "step2")))

	for m := 0; m < 4; m++ {
		setups = setups[:0]
		resp, err := sess.DoRequest(context.Background(), URL(ss.URL), Body(`{"Hello":"World"}`), Setup(setup("request", "step1"), setup("request", "step2")))
		t.Logf("resp=%s, err=%v", resp.Content.String(), err)
		if len(setups) != len(wants) {
			t.Error("len(setups)!= len(setups)")
			return
		}
		for i := range len(setups) {
			if setups[i] != wants[i] {
				t.Errorf("setups=%v, wants=%v", setups[i], wants[i])
				return
			}
			t.Logf("setups=%v, wants=%v", setups[i], wants[i])
		}
	}

}

func TestWarpRoundTripper(t *testing.T) {
	// 测试装饰器链
	var order []string
	rt1 := RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		order = append(order, "rt1")
		return &http.Response{StatusCode: 200}, nil
	})

	rt2 := WarpRoundTripper(rt1)(http.DefaultTransport)
	_, err := rt2.RoundTrip(&http.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 1 || order[0] != "rt1" {
		t.Error("装饰器执行顺序错误")
	}
}

func TestNewTransport(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
		test func(*testing.T, *http.Transport)
	}{
		{
			name: "Unix套接字-有效路径",
			opts: []Option{URL("unix:///tmp/test.sock")},
			test: func(t *testing.T, tr *http.Transport) {
				_, err := tr.DialContext(context.Background(), "unix", "/tmp/test.sock")
				if err == nil {
					t.Error("期望Unix套接字连接失败")
				}
			},
		},
		{
			name: "Unix套接字-无效URL",
			opts: []Option{URL("unix://:::")},
			test: func(t *testing.T, tr *http.Transport) {
				_, err := tr.DialContext(context.Background(), "unix", ":::")
				if err == nil {
					t.Error("期望Unix套接字连接失败")
				}
			},
		},
		{
			name: "本地地址绑定",
			opts: []Option{LocalAddr(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})},
			test: func(t *testing.T, tr *http.Transport) {
				conn, err := tr.DialContext(context.Background(), "tcp", "example.com:80")
				if err == nil {
					conn.Close()
				}
			},
		},
		{
			name: "TLS配置",
			opts: []Option{Verify(false)},
			test: func(t *testing.T, tr *http.Transport) {
				if tr.TLSClientConfig.InsecureSkipVerify != true {
					t.Error("TLS验证配置错误")
				}
			},
		},
		{
			name: "连接池配置",
			opts: []Option{MaxConns(100)},
			test: func(t *testing.T, tr *http.Transport) {
				if tr.MaxIdleConns != 100 || tr.MaxIdleConnsPerHost != 100 {
					t.Error("连接池配置错误")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := newTransport(tt.opts...)
			tt.test(t, tr)
		})
	}
}

func TestTransportWithRealServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // 模拟处理延迟
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	tr := newTransport(
		Timeout(100*time.Millisecond),
		MaxConns(10),
		Verify(false),
	)

	client := &http.Client{Transport: tr}

	// 并发测试
	for range 10 {
		go func() {
			resp, err := client.Get(server.URL)
			if err != nil {
				t.Error(err)
				return
			}
			defer resp.Body.Close()
		}()
	}

	time.Sleep(200 * time.Millisecond)
}

func TestTransportProxy(t *testing.T) {
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("proxy response"))
	}))
	defer proxyServer.Close()

	tr := newTransport(Proxy(proxyServer.URL))

	// 验证代理设置是否生效
	proxyURL, err := tr.Proxy(&http.Request{URL: &url.URL{Scheme: "http", Host: "example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	if proxyURL == nil {
		t.Error("代理未正确设置")
	}
}

// TestNewTransport_DialContext 测试 DialContext 的各种场景
func TestNewTransport_DialContext(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		network     string
		addr        string
		expectError bool
	}{
		{
			name:        "无效Unix socket URL-空格",
			url:         "unix://invalid url with spaces",
			network:     "tcp",
			addr:        "example.com:80",
			expectError: true,
		},
		{
			name:        "有效Unix socket路径",
			url:         "unix:///tmp/test.sock",
			network:     "tcp",
			addr:        "localhost:80",
			expectError: true,
		},
		{
			name:        "无效Unix socket URL-非法字符",
			url:         "unix://::invalid::",
			network:     "tcp",
			addr:        "localhost:80",
			expectError: true,
		},
		{
			name:        "Unix socket URL带端口",
			url:         "unix:///tmp/test.sock:8080",
			network:     "tcp",
			addr:        "localhost:80",
			expectError: true,
		},
		{
			name:    "TCP网络",
			url:     "",
			network: "tcp",
			addr:    "localhost:80",
		},
		{
			name:    "TCP4网络",
			url:     "",
			network: "tcp4",
			addr:    "127.0.0.1:80",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tr *http.Transport
			if tt.url != "" {
				tr = newTransport(URL(tt.url))
			} else {
				tr = newTransport()
			}

			ctx := context.Background()
			_, err := tr.DialContext(ctx, tt.network, tt.addr)

			if tt.expectError && err == nil {
				t.Error("期望出错但成功了")
			}
		})
	}
}
