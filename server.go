package requests

import (
	"bytes"
	"context"
	"net/http"
	"net/http/pprof"
	"time"
)

var notFound = &ServeMux{handler: http.NotFoundHandler()}

// ErrHandler handler err
var ErrHandler = func(err string, code int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, err, code)
	})
}

// WarpHttpHandler warp `http.Handler`.
func WarpHttpHandler(h http.Handler) func(next http.Handler) http.Handler {
	return func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, r)
		})
	}
}

// ServeMux implement ServeHTTP interface.
type ServeMux struct {
	path    string
	handler http.Handler
	opts    []Option
	next    []*ServeMux
}

// ServeHTTP implement http.Handler interface
// 首先对路由进行校验,不满足的话直接404
// 其次执行RequestEach对`http.Request`进行处理,如果处理失败的话，直接返回400
// 最后处理中间件`func(next http.Handler) http.Handler`
func (mux *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	current := notFound
	for _, m := range mux.next {
		if m.path == r.URL.Path {
			current = m
			break
		}
	}

	options := newOptions(mux.opts, current.opts...)
	for _, each := range options.OnRequest {
		if err := each(r.Context(), r); err != nil {
			current.handler = ErrHandler(err.Error(), http.StatusBadRequest)
			break
		}
	}

	h := current.handler
	for i := len(options.HttpHandler) - 1; i >= 0; i-- {
		h = options.HttpHandler[i](h)
	}
	h.ServeHTTP(w, r)

}

// Server server
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

func (s *Server) Run(ctx context.Context) error {
	go func() {
		select {
		case <-ctx.Done():
			if err := s.srv.Shutdown(ctx); err != nil {
				Log("%s http shutdown: %v", time.Now().Format("2006-01-02 15:04:05"), err)
			}
		}
	}()
	Log("%s http serve %s", time.Now().Format("2006-01-02 15:04:05"), s.srv.Addr)
	return s.srv.ListenAndServe()
}

// Route set pattern path to handle
// path cannot override, so if your path not work, maybe it is already exists!
func (s *Server) Route(path string, h http.HandlerFunc, opts ...Option) {
	s.mux.next = append(s.mux.next, &ServeMux{path: path, handler: h, opts: opts})
}

// Use can set middleware which compatible with net/http.ServeMux.
func (s *Server) Use(fn ...func(http.Handler) http.Handler) {
	s.mux.opts = append(s.mux.opts, Use(fn...))
}

func (s *Server) Pprof() {
	s.Route("/debug/pprof/", pprof.Index)
	s.Route("/debug/pprof/allocs", pprof.Index)
	s.Route("/debug/pprof/block", pprof.Index)
	s.Route("/debug/pprof/goroutine", pprof.Index)
	s.Route("/debug/pprof/heap", pprof.Index)
	s.Route("/debug/pprof/mutex", pprof.Index)
	s.Route("/debug/pprof/threadcreate", pprof.Index)
	s.Route("/debug/pprof/cmdline", pprof.Cmdline)
	s.Route("/debug/pprof/profile", pprof.Profile)
	s.Route("/debug/pprof/symbol", pprof.Symbol)
	s.Route("/debug/pprof/trace", pprof.Trace)
}

// ParseBody parse body from `Request.Body`.
func ParseBody(r *http.Request) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r.Body)
	return &buf, err

}
