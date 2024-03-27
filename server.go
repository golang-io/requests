package requests

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// HttpHandlerFunc return a handler function for the given middleware.
type HttpHandlerFunc func(http.HandlerFunc) http.HandlerFunc

// WarpHttpHandler warp handler to handlerFunc
func WarpHttpHandler(h func(http.Handler) http.Handler) HttpHandlerFunc {
	return func(fn http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			h(fn).ServeHTTP(w, r)
		}
	}
}

// WarpHttpHandlerFunc warp handlerFunc to handler
func WarpHttpHandlerFunc(f func(http.HandlerFunc) http.HandlerFunc) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return f(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, r)
		})
	}
}

// ServeMux implement ServeHTTP interface.
type ServeMux struct {
	path    string
	handler http.HandlerFunc
	opts    []Option
	next    []*ServeMux
}

// ServeHTTP implement http.Handler interface
// 首先对路由进行校验,不满足的话直接404
// 其次执行RequestEach对`http.Request`进行处理,如果处理失败的话，直接返回400
// 最后处理中间件`requests.HttpHandlerFunc`
func (mux *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var current *ServeMux
	for _, m := range mux.next {
		if m.path == r.URL.Path {
			current = m
			break
		}
	}

	if current == nil || current.handler == nil {
		http.NotFound(w, r)
		return
	}

	options := newOptions(mux.opts, current.opts...)
	for _, each := range options.OnRequest {
		if err := each(r.Context(), r); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(w, err)
			return
		}
	}
	h := current.handler
	for _, fn := range options.HttpHandlerFunc {
		h = fn(h)
	}
	h(w, r)
}

type Server struct {
	mux *ServeMux
	srv *http.Server
}

// NewServer make server to serve.The options are auto handled.
func NewServer(opts ...Option) *Server {
	options := newOptions(opts)
	mux := &ServeMux{opts: opts}
	return &Server{
		mux: mux,
		srv: &http.Server{
			Addr:    options.URL,
			Handler: mux,
		},
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func (s *Server) Run() error {
	Log("%s http serve %s", time.Now().Format("2006-01-02 15:04:05"), s.srv.Addr)
	return s.srv.ListenAndServe()
}

// Path set pattern to handle
// path cannot override, so if your path not work, maybe it is already exists!
func (s *Server) Path(path string, h func(http.ResponseWriter, *http.Request), opts ...Option) {
	s.mux.next = append(s.mux.next, &ServeMux{path: path, handler: h, opts: opts})
}

// Use can set middleware which compatible with net/http.ServeMux.
func (s *Server) Use(fn ...HttpHandlerFunc) {
	s.mux.opts = append(s.mux.opts, Use(fn...))
}
