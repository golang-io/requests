package requests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/pprof"
	"os"
	"path"
	"regexp"
	"time"
)

var notFound = &ServeMux{handler: http.NotFoundHandler()}

// ErrHandler handler err
var ErrHandler = func(err string, code int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, err, code)
	})
}

// ServeFS serve fs. prefix is route prefix, dir is serve path
// 这里path和prefix都必须以/结尾，否则的话只能处理/backup/a/b/，处理不了/backup/a/b的场景
// eg: r.Route("/backup/", requests.ServeFS("/backup/", "/backup"), Method("GET"))
func ServeFS(prefix, dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix(prefix, http.FileServer(http.Dir(dir))).ServeHTTP(w, r)
	}
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
	path    string
	pattern *regexp.Regexp
	handler http.Handler
	opts    []Option
	next    []*ServeMux
}

// NewServeMux new router.
func NewServeMux(opts ...Option) *ServeMux {
	return &ServeMux{opts: opts}
}

// Route set pattern path to handle
// path cannot override, so if your path not work, maybe it is already exists!
func (mux *ServeMux) Route(path string, h http.HandlerFunc, opts ...Option) {
	mux.next = append(mux.next, &ServeMux{path: path, pattern: regexp.MustCompile(path), handler: h, opts: opts})
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
	current := notFound
	for _, m := range mux.next {
		if m.pattern.MatchString(r.URL.Path) {
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

// Pprof debug
func (mux *ServeMux) Pprof() {
	mux.Route("/debug/pprof/cmdline", pprof.Cmdline)
	mux.Route("/debug/pprof/profile", pprof.Profile)
	mux.Route("/debug/pprof/symbol", pprof.Symbol)
	mux.Route("/debug/pprof/trace", pprof.Trace)
	mux.Route("/debug/pprof/", pprof.Index)
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
				Log("%s http(s) shutdown: %v", time.Now().Format("2006-01-02 15:04:05"), err)
			}
		}
	}()
	Log("%s http(s) serve %s", time.Now().Format("2006-01-02 15:04:05"), s.Addr)
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
