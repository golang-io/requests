package requests

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Options request
type Options struct {
	Method        string
	URL           string
	Path          []string
	Params        map[string]any
	body          any
	Header        http.Header
	Cookies       []http.Cookie
	Timeout       time.Duration
	MaxConns      int
	TraceLv       int
	mLimit        int
	Verify        bool
	Stream        func(int64, []byte) error
	Transport     HttpRoundTripFunc
	RoundTripFunc []func(HttpRoundTripFunc) HttpRoundTripFunc

	OnRequest  []func(context.Context, *http.Request) error
	OnResponse []func(context.Context, *http.Response) error

	// it is only used by server mode
	HttpHandlerFunc []HttpHandlerFunc

	// session used
	LocalAddr net.Addr
	Proxy     func(*http.Request) (*url.URL, error)
}

// Option func
type Option func(*Options)

// NewOptions new request
func newOptions(opts []Option, extends ...Option) Options {
	opt := Options{
		Method:   "GET",
		Params:   make(map[string]any),
		Header:   make(http.Header),
		Timeout:  30 * time.Second,
		MaxConns: 100,
		Proxy:    http.ProxyFromEnvironment,
		mLimit:   1024,
	}
	for _, o := range opts {
		o(&opt)
	}
	for _, o := range extends {
		o(&opt)
	}
	return opt
}

// Method http method
var (
	MethodGet  = Method("GET")
	MethodPost = Method("POST")
)

// MaxConns set max connections
func MaxConns(conn int) Option {
	return func(o *Options) {
		o.MaxConns = conn
	}
}

// Method set method
func Method(method string) Option {
	return func(o *Options) {
		o.Method = method
	}
}

// URL set client to dial connection use http transport or unix socket.
// IF using socket connection. you must set unix in session, and set http in request. For example,
// sess := requests.New(requests.URL("unix:///tmp/requests.sock"))
// sess.DoRequest(context.Background(), requests.URL("http://path?k=v"), requests.Body("12345"))
// https://old.lubui.com/2021/07/26/golang-socket-file/
func URL(url string) Option {
	return func(o *Options) {
		o.URL = url
	}
}

// Path set path
func Path(path string) Option {
	return func(o *Options) {
		o.Path = append(o.Path, path)
	}
}

// Params add query args
func Params(query map[string]any) Option {
	return func(o *Options) {
		for k, v := range query {
			o.Params[k] = v
		}
	}
}

// Param params
func Param(k string, v any) Option {
	return func(o *Options) {
		o.Params[k] = v
	}
}

// Body request body
func Body(body any) Option {
	return func(o *Options) {
		o.body = body
	}
}

// Gzip request gzip compressed
func Gzip(body any) Option {
	reader, err := makeBody(body)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	defer w.Close()

	if _, err := io.Copy(w, reader); err != nil {
		panic(err)
	}

	return func(o *Options) {
		o.body = &buf
		o.Header.Add("Accept-Encoding", "gzip")
		o.Header.Add("Content-Encoding", "gzip")
	}
}

// Form set form, content-type is
func Form(form url.Values) Option {
	return func(o *Options) {
		o.Header.Add("content-type", "application/x-www-form-urlencoded")
		o.body = form
	}
}

// Header header
func Header(k, v string) Option {
	return func(o *Options) {
		o.Header.Add(k, v)
	}
}

// Headers headers
func Headers(kv map[string]string) Option {
	return func(o *Options) {
		for k, v := range kv {
			o.Header.Add(k, v)
		}
	}
}

// Cookie cookie
func Cookie(cookie http.Cookie) Option {
	return func(o *Options) {
		o.Cookies = append(o.Cookies, cookie)
	}
}

// Cookies cookies
func Cookies(cookies ...http.Cookie) Option {
	return func(o *Options) {
		o.Cookies = append(o.Cookies, cookies...)
	}
}

// BasicAuth base auth
func BasicAuth(user, pass string) Option {
	return Header("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user+":"+pass)))

}

// Timeout client timeout duration
func Timeout(timeout time.Duration) Option {
	return func(o *Options) {
		o.Timeout = timeout
	}
}

// Verify verify
func Verify(verify bool) Option {
	return func(o *Options) {
		o.Verify = verify
	}
}

func LocalAddr(addr net.Addr) Option {
	return func(o *Options) {
		o.LocalAddr = addr
	}
}

func Stream(stream func(int64, []byte) error) Option {
	return func(o *Options) {
		o.Stream = stream
	}
}

func RequestEach(each ...func(context.Context, *http.Request) error) Option {
	return func(o *Options) {
		o.OnRequest = append(o.OnRequest, each...)
	}
}

func ResponseEach(each ...func(context.Context, *http.Response) error) Option {
	return func(o *Options) {
		o.OnResponse = append(o.OnResponse, each...)
	}
}

// Host set net/http.Request.Host.
// 在客户端，请求的Host字段（可选地）用来重写请求的Host头。 如过该字段为""，Request.Write方法会使用URL字段的Host。
func Host(host string) Option {
	return func(o *Options) {
		o.RoundTripFunc = append(o.RoundTripFunc, func(fn HttpRoundTripFunc) HttpRoundTripFunc {
			return func(req *http.Request) (*http.Response, error) {
				req.Host = host
				req.Header.Set("Host", host)
				return fn.RoundTrip(req)
			}
		})
	}
}

// Proxy set proxy addr
// os.Setenv("HTTP_PROXY", "http://127.0.0.1:9743")
// os.Setenv("HTTPS_PROXY", "https://127.0.0.1:9743")
// https://stackoverflow.com/questions/14661511/setting-up-proxy-for-http-client
func Proxy(addr string) Option {
	return func(o *Options) {
		if proxyURL, err := url.Parse(addr); err == nil {
			o.Proxy = http.ProxyURL(proxyURL)
		} else {
			panic("parse proxy addr: " + err.Error())
		}
	}
}

// Setup use middleware
func Setup(fn ...func(HttpRoundTripFunc) HttpRoundTripFunc) Option {
	return func(o *Options) {
		o.RoundTripFunc = append(o.RoundTripFunc, fn...)
	}
}

func Use(fn ...HttpHandlerFunc) Option {
	return func(o *Options) {
		o.HttpHandlerFunc = append(o.HttpHandlerFunc, fn...)
	}
}

// RoundTripFunc set default `*http.Transport` by customer define.
func RoundTripFunc(fn HttpRoundTripFunc) Option {
	return func(o *Options) {
		o.Transport = fn
	}
}

// Logf print log
func Logf(f func(ctx context.Context, stat *Stat)) Option {
	return func(o *Options) {
		o.RoundTripFunc = append(o.RoundTripFunc, fprintf(f))
	}
}

// TraceLv Trace
func TraceLv(v int, max ...int) Option {
	return func(o *Options) {
		o.RoundTripFunc = append(o.RoundTripFunc, verbose(v, max...))
	}
}
