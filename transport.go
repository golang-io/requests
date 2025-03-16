package requests

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WarpRoundTripper warp `http.RoundTripper`.
func WarpRoundTripper(next http.RoundTripper) func(http.RoundTripper) http.RoundTripper {
	return func(http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return next.RoundTrip(r)
		})
	}
}

// RoundTripperFunc is a http.RoundTripper implementation, which is a simple function.
type RoundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip implements http.RoundTripper.
func (fn RoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

// printRoundTripper print http client request and response.
func printRoundTripper(f func(ctx context.Context, stat *Stat)) func(http.RoundTripper) http.RoundTripper {
	return func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			resp := newResponse(r)
			resp.Response, resp.Err = next.RoundTrip(r)
			f(r.Context(), resp.Stat())
			return resp.Response, resp.Err
		})
	}
}

// printHandler print http server request and response.
func printHandler(f func(ctx context.Context, stat *Stat)) func(handler http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &ResponseWriter{ResponseWriter: w}
			buf, body, _ := CopyBody(r.Body)
			r.Body = body
			next.ServeHTTP(ww, r)
			f(r.Context(), serveLoad(ww, r, start, buf))
		})
	}
}

type Transport struct {
	opts []Option
	*http.Transport
}

func newTransport(opts ...Option) *Transport {
	options := newOptions(opts)
	return &Transport{
		opts: opts,
		Transport: &http.Transport{
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
			// 上面的例子中有100个goroutine尝试并发的对同一个主机发起http请求，但是连接池只能存放两个连接。
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
		},
	}
}

// RoundTrip implements the [RoundTripper] interface.
// Like the `http.RoundTripper` interface, the error types returned by RoundTrip are unspecified.
func (t *Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	return t.RoundTripper().RoundTrip(r)
}

// RoundTripper return http.RoundTripper.
// Setup: session.Setup -> request.Setup
func (t *Transport) RoundTripper(opts ...Option) http.RoundTripper {
	return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		options := newOptions(t.opts, opts...)
		if options.Transport == nil {
			options.Transport = t.Transport
		}
		for i := len(options.HttpRoundTripper) - 1; i >= 0; i-- {
			options.Transport = options.HttpRoundTripper[i](options.Transport)
		}
		return options.Transport.RoundTrip(r)
	})
}
