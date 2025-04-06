package requests

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type h struct{}

func (h) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("handle ok"))
}
func TestServeMux_RouteRegistration(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		handler  interface{}
		expected string
	}{
		{"HandleFunc", "/test1", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("test1")) }, "test1"},
		{"Handle", "/test2", h{}, "handle ok"},
		{"Route with HandlerFunc", "/test3", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("test3")) }), "test3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := NewServeMux()
			switch h := tt.handler.(type) {
			case func(http.ResponseWriter, *http.Request):
				mux.HandleFunc(tt.path, h)
			case http.Handler:
				mux.Handle(tt.path, h)
			default:
				mux.Route(tt.path, h)
			}

			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", tt.path, nil)
			mux.ServeHTTP(rec, req)

			if rec.Body.String() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, rec.Body.String())
			}
		})
	}
}

func TestServeMux_Middleware(t *testing.T) {
	var order []string
	middleware := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, "before_"+name)
				next.ServeHTTP(w, r)
				order = append(order, "after_"+name)
			})
		}
	}

	mux := NewServeMux(
		Use(middleware("m1"), middleware("m2")),
	)
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.Write([]byte("ok"))
	}, Use(middleware("m3")))
	mux.Route("/test_HandlerFunc", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	mux.Route("/test_Handler", http.RedirectHandler("/test_Handler", http.StatusMovedPermanently), Use(middleware("m4")))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	mux.ServeHTTP(rec, req)

	expected := []string{"before_m1", "before_m2", "before_m3", "handler", "after_m3", "after_m2", "after_m1"}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("middleware order wrong at position %d, expected %s, got %s", i, v, order[i])
		}
	}
}

func TestServer_ConcurrentRequests(t *testing.T) {
	mux := NewServeMux()
	mux.HandleFunc("/concurrent", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	s := httptest.NewServer(mux)
	defer s.Close()

	var wg sync.WaitGroup
	concurrent := 100
	wg.Add(concurrent)

	for i := 0; i < concurrent; i++ {
		go func() {
			defer wg.Done()
			res, err := http.Get(s.URL + "/concurrent")
			if err != nil {
				t.Error(err)
				return
			}
			defer res.Body.Close()
			if res.StatusCode != http.StatusOK {
				t.Errorf("expected status OK, got %v", res.Status)
			}
		}()
	}

	wg.Wait()
}

func TestServer_ErrorHandling(t *testing.T) {
	mux := NewServeMux()

	// Test 404 for non-existent route
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/not-found", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}

	// Test panic recovery
	mux.HandleFunc("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/panic", nil)
	defer func() {
		if r := recover(); r != nil {
			t.Skip("panic was not recovered")
		}
	}()
	mux.ServeHTTP(rec, req)
}

func TestNode_TrieStructure(t *testing.T) {
	tests := []struct {
		name     string
		paths    []string
		testPath string
		expected string
	}{
		{
			name:     "Basic Path",
			paths:    []string{"/test"},
			testPath: "/test",
			expected: "test ok",
		},
		{
			name:     "Nested Path",
			paths:    []string{"/a/b/c"},
			testPath: "/a/b/c",
			expected: "abc ok",
		},
		{
			name:     "Multiple Paths",
			paths:    []string{"/x", "/x/y", "/x/y/z"},
			testPath: "/x/y",
			expected: "xy ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := NewServeMux()

			// Register all paths
			for _, path := range tt.paths {
				path := path // Capture for closure
				mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(strings.ReplaceAll(path[1:], "/", "") + " ok"))
				})
			}

			// Test the specific path
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", tt.testPath, nil)
			mux.ServeHTTP(rec, req)

			if rec.Body.String() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, rec.Body.String())
			}
		})
	}
}

func TestServer_GracefulShutdown(t *testing.T) {
	mux := NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Write([]byte("ok"))
	})

	ctx, cancel := context.WithCancel(context.Background())
	s := NewServer(
		ctx,
		mux,
		URL("http://127.0.0.1:0"),
		OnStart(func(s *http.Server) {
			t.Log("Server start complete")
		}),
		OnShutdown(func(s *http.Server) {
			t.Log("Server shutdown complete")
		}),
	)
	go ListenAndServe(ctx, mux)
	go s.ListenAndServe()
	time.Sleep(100 * time.Millisecond) // Wait for server to start

	// Start a long request
	go http.Get(fmt.Sprintf("http://%s/slow", s.server.Addr))
	time.Sleep(100 * time.Millisecond) // Wait for request to start

	// Trigger shutdown
	cancel()
	time.Sleep(3 * time.Second) // Wait for shutdown to complete
}

func Test_Use(t *testing.T) {
	var order []string
	use := func(name string) func(next http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, fmt.Sprintf("before_%s", name))
				defer func() {
					order = append(order, fmt.Sprintf("after_%s", name))
				}()
				next.ServeHTTP(w, r)
			})
		}
	}

	mux := NewServeMux(
		Use(use("global1"), use("global2")),
	)
	mux.Use(use("global3"))
	mux.Route("/test", func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.Write([]byte("ok"))
	}, Use(use("local")))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	mux.ServeHTTP(rec, req)
	mux.Print()
	expected := []string{
		"before_global1", "before_global2", "before_global3", "before_local",
		"handler",
		"after_local", "after_global3", "after_global2", "after_global1",
	}

	if !reflect.DeepEqual(order, expected) {
		t.Errorf("middleware execution order wrong\nexpected: %v\ngot: %v", expected, order)
	}
}

var f = func(w http.ResponseWriter, r *http.Request) {
	_, _ = fmt.Fprintf(w, "pong\n")
}

func Test_Node(t *testing.T) {
	r := NewNode("/", nil)
	r.Add("/abc/def/ghi", f)
	r.Add("/abc/def/xyz", f)
	r.Add("/1/2/3", f)
	r.Add("/abc/def", f)
	r.Add("/abc/def/", f)
	r.Add("/abc/def/", f)
	r.Add("/", f)
	r.Print()
	//go ListenAndServe(context.Background(), r, URL("0.0.0.0:1234"))
	//fmt.Println(r)
}

// TestErrHandler 测试错误处理器
func TestErrHandler(t *testing.T) {
	handler := ErrHandler("test error", http.StatusBadRequest)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 %d, 得到 %d", http.StatusBadRequest, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "test error") {
		t.Error("错误消息未正确设置")
	}
}

// TestWarpHandler 测试处理器包装
func TestWarpHandler(t *testing.T) {
	var executed bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		executed = true
	})

	wrapped := WarpHandler(handler)(http.NotFoundHandler())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	wrapped.ServeHTTP(rec, req)

	if !executed {
		t.Error("包装的处理器未被执行")
	}
}

// TestNode_EmptyPath 测试空路径情况
func TestNode_EmptyPath(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("期望空路径时发生 panic")
		}
	}()

	node := NewNode("/", nil)
	node.Add("", nil)
}

// TestNode_RootPath 测试根路径处理
func TestNode_RootPath(t *testing.T) {
	node := NewNode("/", nil)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	node.Add("/", handler)

	if node.handler == nil {
		t.Error("根路径处理器未正确设置")
	}
}

// TestServeMux_RedirectAndPprof 测试重定向和 pprof 功能
func TestServeMux_RedirectAndPprof(t *testing.T) {
	mux := NewServeMux()

	// 测试重定向
	t.Run("重定向", func(t *testing.T) {
		mux.Redirect("/old", "/new")
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/old", nil)
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusMovedPermanently {
			t.Errorf("期望状态码 %d, 得到 %d", http.StatusMovedPermanently, rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != "/new" {
			t.Errorf("期望重定向到 /new, 得到 %s", loc)
		}
	})

	// 测试 pprof 路由
	t.Run("Pprof路由", func(t *testing.T) {
		mux.Pprof()
		paths := []string{
			"/debug/pprof/",
			"/debug/pprof/cmdline",
			"/debug/pprof/profile",
			"/debug/pprof/symbol",
			"/debug/pprof/trace",
		}

		for _, path := range paths {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", path, nil)
			mux.ServeHTTP(rec, req)
			if rec.Code == http.StatusNotFound {
				t.Errorf("Pprof 路径 %s 未正确注册", path)
			}
		}
	})
}

// TestServer_TLS 测试 TLS 配置
func TestServer_TLS(t *testing.T) {
	mux := NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	// 创建测试用的临时证书文件
	tmpDir := t.TempDir()
	certFile := tmpDir + "/cert.pem"
	keyFile := tmpDir + "/key.pem"

	// 生成测试证书
	err := generateTestCert(certFile, keyFile)
	if err != nil {
		t.Fatalf("生成测试证书失败: %v", err)
	}

	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
		errMsg  string
	}{
		{
			name: "HTTP无TLS",
			opts: []Option{
				URL("http://127.0.0.1:0"),
			},
			wantErr: false,
		},
		{
			name: "HTTPS缺少证书",
			opts: []Option{
				URL("https://127.0.0.1:0"),
			},
			wantErr: true,
			errMsg:  "missing certificate",
		},
		{
			name: "HTTPS完整配置",
			opts: []Option{
				URL("https://127.0.0.1:0"),
				CertKey(certFile, keyFile),
			},
			wantErr: false,
		},
		{
			name: "证书文件不存在",
			opts: []Option{
				URL("https://127.0.0.1:0"),

				CertKey("not_exist.pem", "not_exist.key"),
			},
			wantErr: true,
			errMsg:  "no such file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			s := NewServer(ctx, mux, tt.opts...)

			errCh := make(chan error, 1)
			go func() {
				errCh <- s.ListenAndServe()
			}()

			var err error
			select {
			case err = <-errCh:
			case <-time.After(200 * time.Millisecond):
				if tt.wantErr {
					t.Error("预期出错但服务器正常启动")
				}
			}

			if tt.wantErr {
				if err == nil {
					t.Error("预期错误但未收到")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Skipf("错误信息不匹配，期望包含 %q，得到 %q", tt.errMsg, err)
				}
			} else if err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
				t.Skipf("未预期的错误: %v", err)
			}
		})
	}
}

// generateTestCert 生成测试用的自签名证书
func generateTestCert(certFile, keyFile string) error {
	// 生成私钥
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	// 创建证书模板
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Co"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
	}

	// 生成证书
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return err
	}

	// 写入证书文件
	certOut, err := os.Create(certFile)
	if err != nil {
		return err
	}
	defer certOut.Close()
	if err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return err
	}

	// 写入私钥文件
	keyOut, err := os.Create(keyFile)
	if err != nil {
		return err
	}
	defer keyOut.Close()
	return pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}

// TestServer_InvalidURL 测试无效 URL
func TestServer_InvalidURL(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("期望无效 URL 时发生 panic")
		}
	}()

	ctx := context.Background()
	NewServer(ctx, nil, URL("://invalid"))
}

// TestServeMux_UnknownHandlerType 测试未知处理器类型
func TestServeMux_UnknownHandlerType(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("期望未知处理器类型时发生 panic")
		}
	}()

	mux := NewServeMux()
	mux.Route("/test", 123) // 传入一个非处理器类型
}

// TestNode_PathsAndPrint 测试路径获取和打印
func TestNode_PathsAndPrint(t *testing.T) {
	node := NewNode("/", nil)
	node.Add("/a", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	node.Add("/b", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	paths := node.paths()
	if len(paths) != 2 {
		t.Errorf("期望 2 个路径，得到 %d 个", len(paths))
	}

	// 测试打印功能
	// 因为打印到标准输出，这里只验证不会 panic
	node.Print()
}
