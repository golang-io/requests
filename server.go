package requests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"net/url"
	"strings"
)

// ErrHandler handler err
var ErrHandler = func(err string, code int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, err, code)
	})
}

// WarpHandler warp `http.Handler`.
func WarpHandler(next http.Handler) func(http.Handler) http.Handler {
	return func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}

// Node trie node
type Node struct {
	path    string
	handler http.Handler
	opts    []Option
	next    map[string]*Node
}

// NewNode new
func NewNode(path string, h http.Handler, opts ...Option) *Node {
	return &Node{path: path, handler: h, opts: opts, next: make(map[string]*Node)}
}

// Add node
func (node *Node) Add(path string, h http.HandlerFunc, opts ...Option) {
	if path == "" {
		panic("path is empty")
	}

	if path == "/" {
		node.handler, node.opts = h, opts
		return
	}
	current := node
	for _, p := range strings.Split(path[1:], "/") {
		if _, ok := current.next[p]; !ok {
			current.next[p] = NewNode(p, http.NotFoundHandler())
		}
		current = current.next[p]
	}
	current.handler, current.opts = h, opts
}

// Find 按照最长的匹配原则，/a/b/c/会优先返回/a/b/c/,其次返回/a/b/c，再返回/a/b，再返回/a，再返回/
func (node *Node) Find(path string) *Node {
	current := node
	for _, p := range strings.Split(path, "/") {
		if next, ok := current.next[p]; !ok {
			break
		} else {
			current = next
		}
	}
	return current
}

func (node *Node) paths() []string {
	var v []string
	for k := range node.next {
		v = append(v, k)
	}
	return v
}

// Print print trie tree struct
func (node *Node) Print() {
	node.print(0)
}

func (node *Node) print(m int) {
	paths := node.paths()
	fmt.Printf("%spath=%s, handler=%v, next=%#v\n", strings.Repeat("    ", m), node.path, node.handler, paths)
	for _, p := range paths {
		node.next[p].print(m + 1)
	}
}

// ServeMux implement ServeHTTP interface.
type ServeMux struct {
	opts []Option
	root *Node
}

// NewServeMux new router.
func NewServeMux(opts ...Option) *ServeMux {
	return &ServeMux{
		opts: opts,
		root: NewNode("/", http.NotFoundHandler()),
	}
}

// Print print trie tree struct.
func (mux *ServeMux) Print() {
	mux.root.Print()
}

// HandleFunc set func pattern path to handle
// path cannot override, so if your path not work, maybe it is already exists!
func (mux *ServeMux) HandleFunc(path string, f func(http.ResponseWriter, *http.Request), opts ...Option) {
	mux.root.Add(path, f, opts...)
}

// Handle set handler pattern path to handler
func (mux *ServeMux) Handle(path string, h http.Handler, opts ...Option) {
	mux.root.Add(path, h.ServeHTTP, opts...)
}

// Route set any pattern path to handle
func (mux *ServeMux) Route(path string, v any, opts ...Option) {
	switch h := v.(type) {
	case http.HandlerFunc:
		mux.HandleFunc(path, h, opts...)
	case http.Handler:
		mux.Handle(path, h, opts...)
	case func(http.ResponseWriter, *http.Request):
		mux.HandleFunc(path, h, opts...)
	default:
		panic("unknown handler type")
	}
}

// Redirect set redirect path to handle
func (mux *ServeMux) Redirect(source, target string) {
	mux.Route(source, http.RedirectHandler(target, http.StatusMovedPermanently).ServeHTTP)
}

// Use can set middleware which compatible with net/http.ServeMux.
func (mux *ServeMux) Use(fn ...func(http.Handler) http.Handler) {
	mux.opts = append(mux.opts, Use(fn...))
}

// ServeHTTP implement http.Handler interface
// 首先对路由进行校验,不满足的话直接404
// 其次执行RequestEach对`http.Request`进行处理,如果处理失败的话，直接返回400
// 最后处理中间件`func(next http.Handler) http.Handler`
func (mux *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	current := mux.root.Find(strings.TrimLeft(r.URL.Path, "/"))
	handler, options := current.handler, newOptions(mux.opts, current.opts...)
	for _, h := range options.HttpHandler {
		handler = h(handler)
	}
	handler.ServeHTTP(w, r)
}

// Pprof debug, 必须使用这个路径访问：/debug/pprof/
func (mux *ServeMux) Pprof() {
	mux.Route("/debug/pprof", pprof.Index)
	mux.Route("/debug/pprof/cmdline", pprof.Cmdline)
	mux.Route("/debug/pprof/profile", pprof.Profile)
	mux.Route("/debug/pprof/symbol", pprof.Symbol)
	mux.Route("/debug/pprof/trace", pprof.Trace)
}

// Server server
type Server struct {
	options Options
	server  *http.Server
}

// NewServer new server, opts is not add to ServeMux
func NewServer(ctx context.Context, h http.Handler, opts ...Option) *Server {
	s := &Server{server: &http.Server{Handler: h}}
	mux, ok := h.(*ServeMux)
	if !ok {
		mux = NewServeMux()
	}
	s.options = newOptions(mux.opts, opts...)

	u, err := url.Parse(s.options.URL)
	if err != nil {
		panic(err)
	}

	s.server.Addr = u.Host
	s.options.OnStart(s.server)
	s.server.RegisterOnShutdown(func() { s.options.OnShutdown(s.server) })
	go s.Shutdown(ctx)
	return s
}

// Shutdown gracefully shuts down the server without interrupting any active connections.
func (s *Server) Shutdown(ctx context.Context) error {
	<-ctx.Done()
	return s.server.Shutdown(ctx)
}

// ListenAndServe listens on the TCP network address srv.Addr and then
// calls [Serve] or [ServeTLS] to handle requests on incoming (TLS) connections.
// Accepted connections are configured to enable TCP keep-alives.
//
// If srv.Addr is blank, ":http" is used.
//
// Filenames containing a certificate and matching private key for the
// server must be provided if neither the [Server]'s TLSConfig.Certificates
// nor TLSConfig.GetCertificate are populated. If the certificate is
// signed by a certificate authority, the certFile should be the
// concatenation of the server's certificate, any intermediates, and
// the CA's certificate.
//
// ListenAndServe(TLS) always returns a non-nil error. After [Server.Shutdown] or
// [Server.Close], the returned error is [ErrServerClosed].
func (s *Server) ListenAndServe() (err error) {
	if s.options.certFile == "" || s.options.keyFile == "" {
		return s.server.ListenAndServe()
	}
	return s.server.ListenAndServeTLS(s.options.certFile, s.options.keyFile)
}

// ListenAndServe listens on the TCP network address addr and then calls
// [Serve] with handler to handle requests on incoming connections.
// Accepted connections are configured to enable TCP keep-alives.
//
// The handler is typically nil, in which case [DefaultServeMux] is used.
//
// ListenAndServe always returns a non-nil error.
func ListenAndServe(ctx context.Context, h http.Handler, opts ...Option) error {
	s := NewServer(ctx, h, opts...)
	return s.ListenAndServe()
}
