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
	Method     string
	URL        string
	Path       []string
	Params     map[string]any
	body       any
	Header     http.Header
	Cookies    []http.Cookie
	Timeout    time.Duration
	MaxConns   int
	TraceLv    int
	TraceLimit int
	Verify     bool
	Logf       func(ctx context.Context, stat Stat)
	Stream     func(int64, []byte) error

	RequestEach  []func(context.Context, *http.Request) error
	ResponseEach []func(context.Context, *http.Response) error

	// session used
	LocalAddr net.Addr
	Hosts     map[string][]string // 内部host文件
	Proxy     func(*http.Request) (*url.URL, error)
}

// Option func
type Option func(*Options)

// NewOptions new request
func newOptions(opts ...Option) Options {
	opt := Options{
		Method:     "GET",
		Params:     make(map[string]any),
		Header:     make(http.Header),
		Timeout:    30 * time.Second,
		MaxConns:   100,
		Hosts:      make(map[string][]string),
		Proxy:      http.ProxyFromEnvironment,
		TraceLimit: 1024,
	}
	for _, o := range opts {
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

// URL set url
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

// TraceLv Trace
func TraceLv(v int, limit ...int) Option {
	return func(o *Options) {
		o.TraceLv = v
		if len(limit) != 0 {
			o.TraceLimit = limit[0]
		}
	}
}

// Verify verify
func Verify(verify bool) Option {
	return func(o *Options) {
		o.Verify = verify
	}
}

func Logf(f func(context.Context, Stat)) Option {
	return func(o *Options) {
		o.Logf = f
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
		o.RequestEach = each
	}
}

func ResponseEach(each ...func(context.Context, *http.Response) error) Option {
	return func(o *Options) {
		o.ResponseEach = each
	}
}

// Hosts 自定义Host配置，参数只能在session级别生效，格式：<host:port>
// 如果存在proxy服务，只能解析代理服务，不能解析url地址
func Hosts(hosts map[string][]string) Option {
	return func(o *Options) {
		o.Hosts = hosts
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
		}
	}
}

// Copy options
func (opt Options) Copy() Options {
	options := newOptions()
	options.Method = opt.Method
	options.URL = opt.URL
	options.Path = append(options.Path, opt.Path...)
	options.Cookies = append(options.Cookies, opt.Cookies...)
	options.body = opt.body
	options.Timeout = opt.Timeout
	options.MaxConns = opt.MaxConns
	options.TraceLv = opt.TraceLv
	options.Verify = opt.Verify
	options.Logf = opt.Logf
	options.LocalAddr = opt.LocalAddr
	options.Stream = opt.Stream
	for k, v := range opt.Params {
		options.Params[k] = v
	}
	for k, v := range opt.Header {
		options.Header.Add(k, v[0])
	}
	options.RequestEach = append(options.RequestEach, opt.RequestEach...)
	options.ResponseEach = append(options.ResponseEach, opt.ResponseEach...)

	return options
}
