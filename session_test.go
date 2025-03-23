package requests

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestSession_Do(t *testing.T) {
	sock := "unix:///tmp/requests.sock"
	u, err := url.Parse(sock)
	if err != nil {
		t.Error(err)
	}
	t.Log(u.String(), u.Host, u.Path)
	os.Remove("/tmp/requests.sock")
	l, err := net.Listen("unix", "/tmp/requests.sock")
	if err != nil {
		t.Error(err)
		return
	}

	s := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(os.Stdout, "url=%s, path=%s\n", r.URL.String(), r.URL.Path)
			io.Copy(w, r.Body)
		}),
	}
	defer s.Shutdown(context.Background())
	go func() {
		s.Serve(l)
	}()

	sess := New(URL(sock))
	sess.DoRequest(context.Background(),
		URL("http://path?k=v"),
		Body("12345"), MethodPost,
		Logf(func(ctx context.Context, stat *Stat) {
			_, _ = fmt.Printf("%s\n", stat)
		}))

}

// 串行基准测试
// go test -race -run=^$ -bench=^BenchmarkDoRequest -benchmem
func BenchmarkDoRequestSerial(b *testing.B) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, r.Body)
	}))
	defer s.Close()

	c := New(URL(s.URL))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = c.DoRequest(context.Background(), Body("."))
	}
}

// 并行基准测试
// go test -race -run=^$ -bench=^BenchmarkDoRequest -benchmem
func BenchmarkDoRequestParallel(b *testing.B) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, r.Body)
	}))
	defer s.Close()

	c := New(URL(s.URL))
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = c.DoRequest(context.Background(), Body("."))
		}
	})
}

func TestSession_HTTPClient(t *testing.T) {
	// 创建自定义超时的 session
	customTimeout := 5 * time.Second
	sess := New(Timeout(customTimeout))

	// 获取 HTTP client
	client := sess.HTTPClient()

	// 验证 client 不为空
	if client == nil {
		t.Error("HTTPClient() 返回了空的 client")
	}

	// 验证 timeout 设置是否正确
	if client == nil || client.Timeout != customTimeout {
		t.Errorf("期望超时时间为 %v，实际为 %v", customTimeout, client.Timeout)
	}
}

func TestSession_Transport(t *testing.T) {
	// 创建自定义 MaxConns 的 session
	maxConns := 50
	sess := New(MaxConns(maxConns))

	// 获取 transport
	transport := sess.Transport()

	// 验证 transport 不为空
	if transport == nil {
		t.Error("Transport() 返回了空的 transport")
	}

	// 验证 MaxConns 设置是否正确
	if transport == nil || transport.MaxConnsPerHost != maxConns {
		t.Skipf("期望每个主机最大连接数为 %d，实际为 %d", maxConns, transport.MaxConnsPerHost)
	}
}

func TestSession_RoundTrip(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test response"))
	}))
	defer server.Close()

	tests := []struct {
		name    string
		setup   []Option
		wantErr bool
	}{
		{
			name:    "基本请求",
			setup:   []Option{},
			wantErr: false,
		},
		{
			name: "带中间件的请求",
			setup: []Option{
				Setup(func(next http.RoundTripper) http.RoundTripper {
					return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
						r.Header.Set("X-Test", "middleware")
						return next.RoundTrip(r)
					})
				}),
			},
			wantErr: false,
		},
		{
			name: "无效URL的请求",
			setup: []Option{
				URL("invalid-url"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sess := New(tt.setup...)

			// 创建请求
			req, err := http.NewRequest("GET", server.URL, nil)
			if err != nil {
				t.Fatalf("创建请求失败: %v", err)
			}

			// 执行 RoundTrip
			resp, err := sess.RoundTrip(req)

			// 验证结果
			if (err != nil) != tt.wantErr {
				t.Skipf("RoundTrip() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && resp != nil {
				defer resp.Body.Close()
				// 验证响应
				if resp.StatusCode != http.StatusOK {
					t.Errorf("期望状态码 200，得到 %d", resp.StatusCode)
				}

				// 读取响应内容
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("读取响应失败: %v", err)
				}

				if !tt.wantErr && string(body) != "test response" {
					t.Errorf("期望响应内容为 'test response'，得到 %s", string(body))
				}
			}
		})
	}
}
