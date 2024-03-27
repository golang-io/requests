package requests

import (
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
	next    *ServeMux
}

// ServeHTTP implement http.Handler interface
// 首先对路由进行校验,不满足的话直接404
// 其次执行RequestEach对`http.Request`进行处理,如果处理失败的话，直接返回400
// 最后处理中间件`requests.HttpHandlerFunc`
func (mux *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	current := mux

	for current != nil && current.path != r.URL.Path {
		current = current.next
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

func (mux *ServeMux) Run(opts ...Option) error {
	options := newOptions(mux.opts, opts...)
	srv := &http.Server{
		Addr:    options.URL,
		Handler: mux,
	}
	Log("%s http serve %s", time.Now().Format("2006-01-02 15:04:05"), options.URL)
	return srv.ListenAndServe()
}

// Root set "" path handler
func (mux *ServeMux) Root(h func(w http.ResponseWriter, r *http.Request)) {
	mux.handler = h
}

// Path set pattern to handle
// the default path is "". it can replace it
// but the other path cannot override, so if your path not work, maybe it is already exists!
// you can set some options effective only used in single uri.
// if you want to use global options, u can set into NewServer.
func (mux *ServeMux) Path(path string, h func(http.ResponseWriter, *http.Request), opts ...Option) {
	current := mux
	for current != nil && current.next != nil {
		current = current.next
		break
	}
	current.next = &ServeMux{path: path, handler: h, opts: opts}
}

// Use can set middleware which compatible with net/http.ServeMux.
func (mux *ServeMux) Use(fn ...HttpHandlerFunc) {
	mux.opts = append(mux.opts, Use(fn...))
}

// NewServer make server to serve.
// the options are auto handled
func NewServer(opts ...Option) *ServeMux {
	mux := &ServeMux{opts: opts} // root node
	return mux
}
