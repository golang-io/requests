package requests

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// emptyBody is an instance of empty reader.
var emptyBody = io.NopCloser(strings.NewReader(""))

// Session httpclient session
// Clients and Transports are safe for concurrent use by multiple goroutines
// for efficiency should only be created once and re-used.
// so, session is also safe for concurrent use by multiple goroutines.
type Session struct {
	opts   []Option
	client *http.Client
}

// New session
func New(opts ...Option) *Session {
	options := newOptions(opts)

	transport := &http.Transport{
		Proxy: options.Proxy,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if strings.HasPrefix(options.URL, "unix://") {
				u, err := url.Parse(options.URL)
				if err != nil {
					return nil, err
				}
				// unix:///tmp/requests.sock => u.Scheme=unix, u.Host=, u.Path=/tmp/requests.sock
				network, addr = u.Scheme, u.Path
			}
			dialer := net.Dialer{
				Timeout:   10 * time.Second, // 限制建立TCP连接的时间
				KeepAlive: 60 * time.Second,
				LocalAddr: options.LocalAddr,
				Resolver: &net.Resolver{
					PreferGo:     true,
					StrictErrors: false,
				},
			}
			return dialer.DialContext(ctx, network, addr)
		},
		MaxIdleConns: options.MaxConns, // 设置连接池的大小为100个连接

		// 默认的DefaultMaxIdleConnsPerHost = 2 这个设置意思时尽管整个连接池是100个连接，但是每个host只有2个。
		// 上面的例子中有100个gooutine尝试并发的对同一个主机发起http请求，但是连接池只能存放两个连接。
		// 所以，第一轮完成请求时，2个连接保持打开状态。但是剩下的98个连接将会被关闭并进入TIME_WAIT状态。
		// 因为这在一个循环中出现，所以会很快就积累上成千上万的TIME_WAIT状态的连接。
		// 最终，会耗尽主机的所有可用端口，从而导致无法打开新的连接。
		MaxIdleConnsPerHost: options.MaxConns,  // 设置每个Host最大的空闲链接
		IdleConnTimeout:     120 * time.Second, // 意味着一个连接在连接池里最多保持120秒的空闲时间，超过这个时间将会被移除并关闭

		//TLSHandshakeTimeout:   10 * time.Second, // 限制 TLS握手的时间
		//ResponseHeaderTimeout: 10 * time.Second, // 限制读取response header的时间
		DisableCompression: true,
		DisableKeepAlives:  false,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !options.Verify,
		},
	}

	s := &Session{
		opts: opts,
		client: &http.Client{
			Timeout:   options.Timeout,
			Transport: transport,
		},
	}
	return s
}

// RoundTrip implements the [RoundTripper] interface.
// Like the `http.RoundTripper` interface, the error types returned by RoundTrip are unspecified.
func (s *Session) RoundTrip(req *http.Request) (*http.Response, error) {
	return s.RoundTripper()(req)
}

// RoundTripper return HttpRoundTripFunc.
// RequestEach: session.RequestEach -> request.RequestEach
// Setup: session.Setup -> request.Setup
// ResponseEach: session.ResponseEach -> request.ResponseEach
func (s *Session) RoundTripper(opts ...Option) HttpRoundTripFunc {
	options := newOptions(s.opts, opts...)
	if options.Transport == nil {
		options.Transport = s.client.Do
	}
	for i := len(options.RoundTripFunc) - 1; i >= 0; i-- { // setup reverse
		options.Transport = options.RoundTripFunc[i](options.Transport)
	}
	return each(options)(options.Transport)
}

// Do send a request and  return `http.Response`. DO NOT forget close `resp.Body`.
func (s *Session) Do(ctx context.Context, opts ...Option) (*http.Response, error) {
	options := newOptions(s.opts, opts...)
	req, err := NewRequestWithContext(ctx, options)
	if err != nil {
		return &http.Response{}, fmt.Errorf("newRequest: %w", err)
	}
	return s.RoundTripper(opts...)(req)
}

// DoRequest send a request and return a response, and is safely close `resp.Body`.
func (s *Session) DoRequest(ctx context.Context, opts ...Option) (*Response, error) {
	options, resp := newOptions(s.opts, opts...), newResponse()
	resp.Request, resp.Err = NewRequestWithContext(ctx, options)
	if resp.Err != nil {
		return resp, resp.Err
	}

	resp.Response, resp.Err = s.RoundTripper(opts...)(resp.Request)
	if resp.Response == nil {
		resp.Response = &http.Response{Body: emptyBody}
	} else if resp.Response.Body == nil {
		resp.Response.Body = emptyBody
	}

	defer resp.Response.Body.Close()

	if options.Stream != nil {
		_, resp.Err = streamRead(resp.Response.Body, options.Stream)
		resp.Content = bytes.NewBufferString("[consumed]")
	} else {
		_, resp.Err = resp.Content.ReadFrom(resp.Response.Body)
		resp.Response.Body = io.NopCloser(bytes.NewReader(resp.Content.Bytes()))
	}
	return resp, resp.Err
}
