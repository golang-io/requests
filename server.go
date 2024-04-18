package requests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"path"
	"strings"
)

// ErrHandler handler err
var ErrHandler = func(err string, code int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, err, code)
	})
}

// ServeUpload serve upload handler.
// curl http://127.0.0.1:8080/_upload H 'Content-Type: multipart/form-data' -F '/abc/123=@xxx.txt' -F '456=@abc/xyz.txt'
// upload files is $perfix/abc/123/xxx.txt and $perfix/456/xyz.txt
func ServeUpload(prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reader, err := r.MultipartReader()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		save := func(prefix, dir, file string, r io.Reader) error {
			if file == "" { // this is FormData
				data, err := io.ReadAll(r)
				fmt.Printf("FormData=[%s]\n", string(data))
				return err
			}
			paths := path.Join(prefix, dir)
			if err := os.MkdirAll(paths, 0755); err != nil {
				return err
			}
			dst, err := os.Create(path.Join(paths, file))
			if err != nil {
				return err
			}
			defer dst.Close()
			_, err = io.Copy(dst, r)
			return err
		}

		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			//fmt.Printf("FileName=[%s], FormName=[%s]\n", part.FileName(), part.FormName())
			if err := save(prefix, part.FormName(), part.FileName(), part); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		_, _ = fmt.Fprintf(w, "Successfully Uploaded File\n")
	}
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
	opts []Option
	Root *Node
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

	current := node
	for _, p := range strings.Split(path[1:], "/") {
		if _, ok := current.next[p]; !ok {
			current.next[p] = NewNode(p, http.NotFoundHandler())
		}
		current = current.next[p]
	}
	current.handler, current.opts = h, opts

}

// Find node
// 按照最长的匹配原则，/a/b/c/会优先返回/a/b/c/,其次返回/a/b/c，再返回/a/b，再返回/a，再返回/
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
	fmt.Printf("%spath=%s, handler=%v, next=%#v\n", strings.Repeat("\t", m), node.path, node.handler, paths)
	for _, p := range paths {
		node.next[p].print(m + 1)
	}
}

// NewServeMux new router.
func NewServeMux(opts ...Option) *ServeMux {
	return &ServeMux{opts: opts, Root: NewNode("/", http.NotFoundHandler())}
}

// Route set pattern path to handle
// path cannot override, so if your path not work, maybe it is already exists!
func (mux *ServeMux) Route(path string, h http.HandlerFunc, opts ...Option) {
	mux.Root.Add(path, h, opts...)
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
	current := mux.Root.Find(r.URL.Path[1:])

	options := newOptions(mux.opts, current.opts...)
	for _, each := range options.OnRequest {
		if err := each(r.Context(), r); err != nil {
			current.handler = ErrHandler(err.Error(), http.StatusBadRequest)
			break
		}
	}

	handler := current.handler
	for _, h := range options.HttpHandler {
		handler = h(handler)
	}
	handler.ServeHTTP(w, r)
}

// Pprof debug
func (mux *ServeMux) Pprof() {
	mux.Route("/debug/pprof", pprof.Index)
	mux.Route("/debug/pprof/cmdline", pprof.Cmdline)
	mux.Route("/debug/pprof/profile", pprof.Profile)
	mux.Route("/debug/pprof/symbol", pprof.Symbol)
	mux.Route("/debug/pprof/trace", pprof.Trace)
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
func ListenAndServe(ctx context.Context, h http.Handler, opts ...Option) error {
	mux, _ := h.(*ServeMux)
	options := newOptions(mux.opts, opts...)
	s := &http.Server{Addr: options.URL, Handler: h}

	go func() {
		select {
		case <-ctx.Done():
			if err := s.Shutdown(ctx); err != nil {
				log.Println("http(s) shutdown: ", err)
			}
		}
	}()
	log.Println("http(s) serve", s.Addr)
	if options.certFile == "" || options.keyFile == "" {
		return s.ListenAndServe()
	}
	return s.ListenAndServeTLS(options.certFile, options.keyFile)
}

// ParseBody parse body from `Request.Body`.
func ParseBody(r *http.Request) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r.Body)
	return &buf, err
}
