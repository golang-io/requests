package requests

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Session httpclient session
// Clients and Transports are safe for concurrent use by multiple goroutines
// for efficiency should only be created once and re-used.
// so, session is also safe for concurrent use by multiple goroutines.
type Session struct {
	opts []Option
	*http.Transport
	*http.Client
}

// New session
func New(opts ...Option) *Session {

	options := newOptions(opts...)

	transport := &http.Transport{
		Proxy: options.Proxy,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			tmp, ok := options.Hosts[addr]
			if ok {
				addr = tmp[0]
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
		opts:      opts,
		Transport: transport,
		Client: &http.Client{
			Timeout:   options.Timeout,
			Transport: transport,
		},
	}
	return s
}

//func (s *Session) Proxy(addr string, auth *proxy.Auth) error {
//	proxyURL, err := url.Parse(addr)
//	if err != nil {
//		return err
//	}
//	switch proxyURL.Scheme {
//	case "http", "https":
//		s.Transport.Proxy = http.ProxyURL(proxyURL)
//	case "socks5", "socks4":
//		s.Transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
//			dialer, err := proxy.SOCKS5("tcp", proxyURL.Host, auth, proxy.Direct)
//			if err != nil {
//				return nil, err
//			}
//			return dialer.Dial(network, addr)
//		}
//	default:
//		return fmt.Errorf("proxy scheme[%s] invalid", proxyURL.Scheme)
//	}
//	return nil
//}

func (s *Session) RoundTrip(opts ...Option) http.RoundTripper {
	return &Transport{client: s.Client, options: withOptions(s.opts, opts)}
}

// DoRequest send a request and return a response
func (s *Session) DoRequest(ctx context.Context, opts ...Option) (*Response, error) {
	tr := &Transport{client: s.Client, options: withOptions(s.opts, opts)}
	req, err := NewRequestWithContext(ctx, tr.options)
	if err != nil {
		return nil, fmt.Errorf("newRequest: %w", err)
	}
	return tr.request(req)
}

// Upload upload file
//func (s *Session) Upload(url, file string) (*Response, error) {
//	f, err := os.Open(file)
//	if err != nil {
//		return nil, err
//	}
//	defer f.Close()
//	return s.Post(url, "binary/octet-stream", f)
//}

// Uploadmultipart upload with multipart form
//func (s *Session) Uploadmultipart(url, file string, fields map[string]string) (*Response, error) {
//	f, err := os.Open(file)
//	if err != nil {
//		return nil, err
//	}
//	defer func(f *os.File) {
//		_ = f.Close()
//	}(f)
//
//	body := &bytes.Buffer{}
//	writer := multipart.NewWriter(body)
//	fw, err := writer.CreateFormFile("file", fields["filename"])
//	if err != nil {
//		return nil, fmt.Errorf("CreateFormFile %v", err)
//	}
//
//	_, err = io.Copy(fw, f)
//	if err != nil {
//		return nil, fmt.Errorf("copying fileWriter %v", err)
//	}
//	for k, v := range fields {
//		if err = writer.WriteField(k, v); err != nil {
//			return nil, err
//		}
//	}
//
//	err = writer.Close() // close writer before POST request
//	if err != nil {
//		return nil, fmt.Errorf("writerClose: %v", err)
//	}
//
//	return s.Post(url, writer.FormDataContentType(), body)
//}
