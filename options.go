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
	Method   string
	URL      string
	Path     []string
	RawQuery url.Values
	body     any
	Header   http.Header
	Cookies  []http.Cookie
	Timeout  time.Duration
	MaxConns int
	Verify   bool
	Stream   func(int64, []byte) error

	Transport        http.RoundTripper
	HttpRoundTripper []func(http.RoundTripper) http.RoundTripper

	Handler     http.Handler
	HttpHandler []func(http.Handler) http.Handler

	certFile string
	keyFile  string

	// client session used
	LocalAddr net.Addr
	Proxy     func(*http.Request) (*url.URL, error)
}

// Option func
type Option func(*Options)

// NewOptions new request
func newOptions(opts []Option, extends ...Option) Options {
	opt := Options{
		Method:   "GET",
		RawQuery: make(url.Values),
		Header:   make(http.Header),
		Timeout:  30 * time.Second,
		MaxConns: 100,
		Proxy:    http.ProxyFromEnvironment,
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

// CertKey is cert and key file.
func CertKey(cert, key string) Option {
	return func(o *Options) {
		o.certFile, o.keyFile = cert, key
	}
}

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
func Params(query map[string]string) Option {
	return func(o *Options) {
		for k, v := range query {
			o.RawQuery.Add(k, v)
		}
	}
}

// Param params
func Param(k string, v ...string) Option {
	return func(o *Options) {
		for _, x := range v {
			o.RawQuery.Add(k, x)
		}
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
func BasicAuth(username, password string) Option {
	return Header("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))

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

// LocalAddr local ip
// for tcp: &net.TCPAddr{IP: ip}
// for unix: &net.UnixAddr{Net: "unix", Name: "xxx")}
func LocalAddr(addr net.Addr) Option {
	return func(o *Options) {
		o.LocalAddr = addr
	}
}

// Stream handle func
func Stream(stream func(int64, []byte) error) Option {
	return func(o *Options) {
		o.Stream = stream
	}
}

// Host set net/http.Request.Host.
// 在客户端，请求的Host字段（可选地）用来重写请求的Host头。 如过该字段为""，Request.Write方法会使用URL字段的Host。
func Host(host string) Option {
	return func(o *Options) {
		o.HttpRoundTripper = append(o.HttpRoundTripper, func(next http.RoundTripper) http.RoundTripper {
			return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
				r.Host = host
				r.Header.Set("Host", host)
				return next.RoundTrip(r)
			})
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

// Setup is used for client middleware
func Setup(fn ...func(tripper http.RoundTripper) http.RoundTripper) Option {
	return func(o *Options) {
		for _, f := range fn {
			o.HttpRoundTripper = append([]func(http.RoundTripper) http.RoundTripper{f}, o.HttpRoundTripper...)
		}
	}
}

// Use is used for server middleware
func Use(fn ...func(http.Handler) http.Handler) Option {
	return func(o *Options) {
		for _, f := range fn {
			o.HttpHandler = append([]func(http.Handler) http.Handler{f}, o.HttpHandler...)
		}
	}
}

// RoundTripper set default `*http.Transport` by customer define.
func RoundTripper(tr http.RoundTripper) Option {
	return func(o *Options) {
		o.Transport = tr
	}
}

// Logf print log
func Logf(f func(ctx context.Context, stat *Stat)) Option {
	return func(o *Options) {
		o.HttpRoundTripper = append(o.HttpRoundTripper, printRoundTripper(f))
		o.HttpHandler = append(o.HttpHandler, printHandler(f))

	}
}
