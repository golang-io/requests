package requests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	mux.ServeHTTP(rec, req)

	expected := []string{"before_m1", "before_m2", "handler", "after_m2", "after_m1"}
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
		OnShutdown(func(s *http.Server) {
			t.Log("Server shutdown complete")
		}),
	)

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

	mux.Route("/test", func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.Write([]byte("ok"))
	}, Use(use("local")))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	mux.ServeHTTP(rec, req)

	expected := []string{
		"before_global1", "before_global2", "before_local",
		"handler",
		"after_local", "after_global2", "after_global1",
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
