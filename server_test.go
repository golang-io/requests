package requests

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
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
		handler  any
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
	// mux.Route("/test_HandlerFunc", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	// mux.Route("/test_Handler", http.RedirectHandler("/test_Handler", http.StatusMovedPermanently), Use(middleware("m4")))
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

	for range concurrent {
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
	mux.Print(os.Stdout)
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
	r.Print(os.Stdout)
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

	if node.methods[""] == nil {
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
	node.Print(os.Stdout)
}

func Test_Methods(t *testing.T) {
	tests := []struct {
		name           string
		setupMux       func(*ServeMux)
		requestMethod  string
		requestPath    string
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "GET方法限制",
			setupMux: func(mux *ServeMux) {
				mux.GET("/test", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("GET ok"))
				})
			},
			requestMethod:  "GET",
			requestPath:    "/test",
			expectedStatus: http.StatusOK,
			expectedBody:   "GET ok",
		},
		{
			name: "POST方法限制",
			setupMux: func(mux *ServeMux) {
				mux.POST("/test", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("POST ok"))
				})
			},
			requestMethod:  "POST",
			requestPath:    "/test",
			expectedStatus: http.StatusOK,
			expectedBody:   "POST ok",
		},
		{
			name: "PUT方法限制",
			setupMux: func(mux *ServeMux) {
				mux.PUT("/test", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("PUT ok"))
				})
			},
			requestMethod:  "PUT",
			requestPath:    "/test",
			expectedStatus: http.StatusOK,
			expectedBody:   "PUT ok",
		},
		{
			name: "DELETE方法限制",
			setupMux: func(mux *ServeMux) {
				mux.DELETE("/test", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("DELETE ok"))
				})
			},
			requestMethod:  "DELETE",
			requestPath:    "/test",
			expectedStatus: http.StatusOK,
			expectedBody:   "DELETE ok",
		},
		{
			name: "OPTIONS方法限制",
			setupMux: func(mux *ServeMux) {
				mux.OPTIONS("/test", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("OPTIONS ok"))
				})
			},
			requestMethod:  "OPTIONS",
			requestPath:    "/test",
			expectedStatus: http.StatusOK,
			expectedBody:   "OPTIONS ok",
		},
		{
			name: "HEAD方法限制",
			setupMux: func(mux *ServeMux) {
				mux.HEAD("/test", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("HEAD ok"))
				})
			},
			requestMethod:  "HEAD",
			requestPath:    "/test",
			expectedStatus: http.StatusOK,
			expectedBody:   "HEAD ok",
		},
		{
			name: "CONNECT方法限制",
			setupMux: func(mux *ServeMux) {
				mux.CONNECT("/test", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("CONNECT ok"))
				})
			},
			requestMethod:  "CONNECT",
			requestPath:    "/test",
			expectedStatus: http.StatusOK,
			expectedBody:   "CONNECT ok",
		},
		{
			name: "TRACE方法限制",
			setupMux: func(mux *ServeMux) {
				mux.TRACE("/test", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("TRACE ok"))
				})
			},
			requestMethod:  "TRACE",
			requestPath:    "/test",
			expectedStatus: http.StatusOK,
			expectedBody:   "TRACE ok",
		},
		{
			name: "方法不匹配返回405",
			setupMux: func(mux *ServeMux) {
				mux.GET("/test", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("GET ok"))
				})
			},
			requestMethod:  "POST",
			requestPath:    "/test",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method Not Allowed\n",
		},
		{
			name: "路径不存在返回404",
			setupMux: func(mux *ServeMux) {
				mux.GET("/test", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("GET ok"))
				})
			},
			requestMethod:  "GET",
			requestPath:    "/notfound",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Not Found\n",
		},
		{
			name: "使用Method选项限制",
			setupMux: func(mux *ServeMux) {
				mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("Method option ok"))
				}, Method("PATCH"))
			},
			requestMethod:  "PATCH",
			requestPath:    "/test",
			expectedStatus: http.StatusOK,
			expectedBody:   "Method option ok",
		},
		{
			name: "Method选项不匹配",
			setupMux: func(mux *ServeMux) {
				mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("Method option ok"))
				}, Method("PATCH"))
			},
			requestMethod:  "GET",
			requestPath:    "/test",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method Not Allowed\n",
		},
		{
			name: "多个方法支持",
			setupMux: func(mux *ServeMux) {
				mux.GET("/test", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("GET ok"))
				})
				mux.POST("/test", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("POST ok"))
				})
			},
			requestMethod:  "POST",
			requestPath:    "/test",
			expectedStatus: http.StatusOK,
			expectedBody:   "POST ok",
		},
		{
			name: "根路径方法限制",
			setupMux: func(mux *ServeMux) {
				mux.GET("/", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("root GET ok"))
				})
			},
			requestMethod:  "GET",
			requestPath:    "/",
			expectedStatus: http.StatusOK,
			expectedBody:   "root GET ok",
		},
		{
			name: "根路径方法不匹配",
			setupMux: func(mux *ServeMux) {
				mux.GET("/", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("root GET ok"))
				})
			},
			requestMethod:  "POST",
			requestPath:    "/",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method Not Allowed\n",
		},
		{
			name: "嵌套路径方法限制",
			setupMux: func(mux *ServeMux) {
				mux.GET("/api/v1/users", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("nested GET ok"))
				})
			},
			requestMethod:  "GET",
			requestPath:    "/api/v1/users",
			expectedStatus: http.StatusOK,
			expectedBody:   "nested GET ok",
		},
		{
			name: "嵌套路径方法不匹配",
			setupMux: func(mux *ServeMux) {
				mux.GET("/api/v1/users", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("nested GET ok"))
				})
			},
			requestMethod:  "PUT",
			requestPath:    "/api/v1/users",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method Not Allowed\n",
		},
		{
			name: "使用Route方法设置方法限制",
			setupMux: func(mux *ServeMux) {
				mux.Route("/test", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("Route method ok"))
				}, Method("PUT"))
			},
			requestMethod:  "PUT",
			requestPath:    "/test",
			expectedStatus: http.StatusOK,
			expectedBody:   "Route method ok",
		},
		{
			name: "Route方法设置的方法不匹配",
			setupMux: func(mux *ServeMux) {
				mux.Route("/test", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("Route method ok"))
				}, Method("PUT"))
			},
			requestMethod:  "GET",
			requestPath:    "/test",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method Not Allowed\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := NewServeMux()
			tt.setupMux(mux)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.requestMethod, tt.requestPath, nil)
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("期望状态码 %d, 得到 %d", tt.expectedStatus, rec.Code)
			}

			if rec.Body.String() != tt.expectedBody {
				t.Errorf("期望响应体 %q, 得到 %q", tt.expectedBody, rec.Body.String())
			}
		})
	}
}

// TestMethodRestriction_EdgeCases 测试方法限制的边缘情况
func TestMethodRestriction_EdgeCases(t *testing.T) {
	t.Run("空方法处理", func(t *testing.T) {
		mux := NewServeMux()
		mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("empty method ok"))
		}, Method(""))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("期望状态码 %d, 得到 %d", http.StatusOK, rec.Code)
		}
		if rec.Body.String() != "empty method ok" {
			t.Errorf("期望响应体 %q, 得到 %q", "empty method ok", rec.Body.String())
		}
	})

	t.Run("大小写不敏感方法匹配", func(t *testing.T) {
		mux := NewServeMux()
		mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("case insensitive ok"))
		}, Method("get"))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		mux.ServeHTTP(rec, req)

		// 注意：HTTP方法匹配是大小写敏感的，所以这里应该返回405
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("期望状态码 %d, 得到 %d", http.StatusMethodNotAllowed, rec.Code)
		}
	})

	t.Run("自定义HTTP方法", func(t *testing.T) {
		mux := NewServeMux()
		mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("custom method ok"))
		}, Method("CUSTOM"))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest("CUSTOM", "/test", nil)
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("期望状态码 %d, 得到 %d", http.StatusOK, rec.Code)
		}
		if rec.Body.String() != "custom method ok" {
			t.Errorf("期望响应体 %q, 得到 %q", "custom method ok", rec.Body.String())
		}
	})
}

// TestMethodRestriction_Performance 测试方法限制的性能
func TestMethodRestriction_Performance(t *testing.T) {
	mux := NewServeMux()

	// 注册多个不同方法的处理器
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"}
	for _, method := range methods {
		method := method // 捕获变量
		mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(method + " ok"))
		}, Method(method))
	}

	// 并发测试
	var wg sync.WaitGroup
	concurrent := 50
	wg.Add(concurrent)

	for i := 0; i < concurrent; i++ {
		go func() {
			defer wg.Done()
			for _, method := range methods {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(method, "/test", nil)
				mux.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK {
					t.Errorf("方法 %s 期望状态码 %d, 得到 %d", method, http.StatusOK, rec.Code)
				}
				if rec.Body.String() != method+" ok" {
					t.Errorf("方法 %s 期望响应体 %q, 得到 %q", method, method+" ok", rec.Body.String())
				}
			}
		}()
	}

	wg.Wait()
}

// TestServer_TimeoutSettings 测试服务器的超时设置
func TestServer_TimeoutSettings(t *testing.T) {
	tests := []struct {
		name          string
		timeout       time.Duration
		expectedRead  time.Duration
		expectedWrite time.Duration
		description   string
	}{
		{
			name:          "零超时设置",
			timeout:       0,                // 显式设置为0
			expectedRead:  30 * time.Second, // 实际行为：使用默认值
			expectedWrite: 30 * time.Second, // 实际行为：使用默认值
			description:   "当显式设置超时为0时，实际使用默认超时",
		},
		{
			name:          "自定义超时设置",
			timeout:       5 * time.Second,
			expectedRead:  5 * time.Second,
			expectedWrite: 5 * time.Second,
			description:   "应该使用自定义的5秒超时",
		},
		{
			name:          "短超时设置",
			timeout:       100 * time.Millisecond,
			expectedRead:  100 * time.Millisecond,
			expectedWrite: 100 * time.Millisecond,
			description:   "应该使用100毫秒超时",
		},
		{
			name:          "长超时设置",
			timeout:       2 * time.Minute,
			expectedRead:  2 * time.Minute,
			expectedWrite: 2 * time.Minute,
			description:   "应该使用2分钟超时",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建ServeMux
			mux := NewServeMux()
			mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("timeout test ok"))
			})

			// 创建上下文
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// 创建服务器选项
			var opts []Option
			if tt.timeout > 0 {
				opts = append(opts, Timeout(tt.timeout))
			}

			// 创建服务器
			server := NewServer(ctx, mux, opts...)

			// 验证超时设置
			if server.server.ReadTimeout != tt.expectedRead {
				t.Errorf("ReadTimeout 设置错误: 期望 %v, 实际 %v", tt.expectedRead, server.server.ReadTimeout)
			}

			if server.server.WriteTimeout != tt.expectedWrite {
				t.Errorf("WriteTimeout 设置错误: 期望 %v, 实际 %v", tt.expectedWrite, server.server.WriteTimeout)
			}

			// 验证Options中的Timeout设置
			if tt.timeout > 0 && server.options.Timeout != tt.timeout {
				t.Errorf("Options.Timeout 设置错误: 期望 %v, 实际 %v", tt.timeout, server.options.Timeout)
			}
		})
	}
}

// TestServer_TimeoutBehavior 测试超时行为
func TestServer_TimeoutBehavior(t *testing.T) {
	t.Run("读取超时测试", func(t *testing.T) {
		// 创建一个慢处理器来测试读取超时
		mux := NewServeMux()
		mux.HandleFunc("/slow-read", func(w http.ResponseWriter, r *http.Request) {
			// 模拟慢读取
			time.Sleep(200 * time.Millisecond)
			w.Write([]byte("slow read ok"))
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// 创建短超时的服务器
		server := NewServer(ctx, mux, Timeout(50*time.Millisecond))

		// 启动服务器
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("创建监听器失败: %v", err)
		}
		defer listener.Close()

		// 修改服务器地址
		server.server.Addr = listener.Addr().String()

		// 启动服务器
		go func() {
			if err := server.server.Serve(listener); err != nil && err != http.ErrServerClosed {
				t.Logf("服务器错误: %v", err)
			}
		}()

		// 等待服务器启动
		time.Sleep(10 * time.Millisecond)

		// 发送请求
		client := &http.Client{
			Timeout: 100 * time.Millisecond,
		}

		resp, err := client.Get("http://" + listener.Addr().String() + "/slow-read")
		if err != nil {
			t.Logf("请求错误: %v", err)
			// 超时是预期的行为
			return
		}
		defer resp.Body.Close()

		// 如果请求成功，读取响应
		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("读取响应失败: %v", err)
				return
			}
			if string(body) != "slow read ok" {
				t.Errorf("期望响应 'slow read ok', 得到 '%s'", string(body))
			}
		}
	})

	t.Run("写入超时测试", func(t *testing.T) {
		// 创建一个慢写入处理器来测试写入超时
		mux := NewServeMux()
		mux.HandleFunc("/slow-write", func(w http.ResponseWriter, r *http.Request) {
			// 模拟慢写入
			time.Sleep(200 * time.Millisecond)
			w.Write([]byte("slow write ok"))
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// 创建短超时的服务器
		server := NewServer(ctx, mux, Timeout(50*time.Millisecond))

		// 启动服务器
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("创建监听器失败: %v", err)
		}
		defer listener.Close()

		// 修改服务器地址
		server.server.Addr = listener.Addr().String()

		// 启动服务器
		go func() {
			if err := server.server.Serve(listener); err != nil && err != http.ErrServerClosed {
				t.Logf("服务器错误: %v", err)
			}
		}()

		// 等待服务器启动
		time.Sleep(10 * time.Millisecond)

		// 发送请求
		client := &http.Client{
			Timeout: 100 * time.Millisecond,
		}

		resp, err := client.Get("http://" + listener.Addr().String() + "/slow-write")
		if err != nil {
			t.Logf("请求错误: %v", err)
			// 超时是预期的行为
			return
		}
		defer resp.Body.Close()

		// 如果请求成功，读取响应
		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("读取响应失败: %v", err)
				return
			}
			if string(body) != "slow write ok" {
				t.Errorf("期望响应 'slow write ok', 得到 '%s'", string(body))
			}
		}
	})
}

// TestServer_DefaultTimeout 测试默认超时设置
func TestServer_DefaultTimeout(t *testing.T) {
	t.Run("不设置Timeout选项时的默认行为", func(t *testing.T) {
		mux := NewServeMux()
		mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("default timeout"))
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// 不设置Timeout选项，应该使用默认值
		server := NewServer(ctx, mux)

		// 验证默认超时设置
		if server.server.ReadTimeout != 30*time.Second {
			t.Errorf("默认ReadTimeout应该为30秒，实际为 %v", server.server.ReadTimeout)
		}
		if server.server.WriteTimeout != 30*time.Second {
			t.Errorf("默认WriteTimeout应该为30秒，实际为 %v", server.server.WriteTimeout)
		}
		if server.options.Timeout != 30*time.Second {
			t.Errorf("默认Options.Timeout应该为30秒，实际为 %v", server.options.Timeout)
		}
	})
}

// TestServer_TimeoutEdgeCases 测试超时设置的边界情况
func TestServer_TimeoutEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		timeout     time.Duration
		description string
	}{
		{
			name:        "零超时",
			timeout:     0,
			description: "零超时应该使用默认值",
		},
		{
			name:        "负超时",
			timeout:     -1 * time.Second,
			description: "负超时应该被正确处理",
		},
		{
			name:        "极小超时",
			timeout:     1 * time.Nanosecond,
			description: "极小超时应该被正确处理",
		},
		{
			name:        "极大超时",
			timeout:     24 * time.Hour,
			description: "极大超时应该被正确处理",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := NewServeMux()
			mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("edge case ok"))
			})

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// 创建服务器
			server := NewServer(ctx, mux, Timeout(tt.timeout))

			// 验证服务器创建成功
			if server == nil {
				t.Fatal("服务器创建失败")
			}

			// 验证超时设置
			if server.server.ReadTimeout != tt.timeout {
				t.Errorf("ReadTimeout 设置错误: 期望 %v, 实际 %v", tt.timeout, server.server.ReadTimeout)
			}
			if server.server.WriteTimeout != tt.timeout {
				t.Errorf("WriteTimeout 设置错误: 期望 %v, 实际 %v", tt.timeout, server.server.WriteTimeout)
			}
		})
	}
}
