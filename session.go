package requests

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// Session httpclient session
// Clients and Transports are safe for concurrent use by multiple goroutines
// for efficiency should only be created once and re-used.
// so, session is also safe for concurrent use by multiple goroutines.
type Session struct {
	*http.Transport
	*http.Client
	options Options
	wg      sync.Mutex
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

		// TLSHandshakeTimeout:   10 * time.Second, // 限制 TLS握手的时间
		// ResponseHeaderTimeout: 60 * time.Second, // 限制读取response header的时间
		DisableCompression: true,
		DisableKeepAlives:  false,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !options.Verify,
		},
	}

	s := &Session{
		Transport: transport,
		Client: &http.Client{
			Timeout:   options.Timeout,
			Transport: transport,
		},
		options: options,
	}
	return s
}

// Init init
func (s *Session) Init(opts ...Option) {
	for _, o := range opts {
		o(&s.options)
	}
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

// Timeout set client timeout
func (s *Session) Timeout(timeout time.Duration) *Session {
	s.Client.Timeout = timeout
	return sess
}

// SetKeepAlives set transport disableKeepAlives default transport is keepalive,
// if set false, only use the connection to the server for a single HTTP request.
func (s *Session) SetKeepAlives(keepAlives bool) *Session {
	s.Transport.DisableKeepAlives = !keepAlives
	return sess
}

func (s *Session) WithOption(opts ...Option) *Session {
	s.wg.Lock()
	defer s.wg.Unlock()
	for _, o := range opts {
		o(&s.options)
	}
	return sess
}

func (s *Session) copyOption(opts ...Option) Options {
	s.wg.Lock()
	defer s.wg.Unlock()
	options := s.options.Copy()
	for _, o := range opts {
		o(&options)
	}
	return options
}

// DoRequest send a request and return a response
func (s *Session) DoRequest(ctx context.Context, opts ...Option) (*Response, error) {
	options, resp := s.copyOption(opts...), Response{StartAt: time.Now()}

	defer func(resp *Response) {
		resp.Cost = time.Since(resp.StartAt)
		if options.Logf != nil {
			options.Logf(ctx, resp.Stat())
		}
	}(&resp)

	resp.Request, resp.Err = NewRequestWithContext(ctx, options)
	if resp.Err != nil {
		return &resp, fmt.Errorf("newRequest: %w", resp.Err)
	}

	if options.TraceLv != 0 {
		resp.Response, resp.Err = s.DebugTrace(resp.Request, options.TraceLv, options.TraceLimit)
	} else {
		resp.Response, resp.Err = s.Client.Do(resp.Request)
	}

	if resp.Err != nil {
		return &resp, fmt.Errorf("doRequest: %w", resp.Err)
	}

	if resp.Response == nil || resp.Response.Body == nil {
		resp.Err = fmt.Errorf("resp.Body is nil")
		return &resp, resp.Err
	}

	defer resp.Response.Body.Close()

	for _, each := range options.ResponseEach {
		if err := each(ctx, resp.Response); err != nil {
			return &resp, fmt.Errorf("responseEach: %w", err)
		}
	}

	if options.Stream != nil {
		resp.Response.ContentLength, resp.Err = resp.stream(options.Stream)
	} else {
		resp.Response.ContentLength, resp.Err = resp.body.ReadFrom(resp.Response.Body)
	}

	return &resp, resp.Err
}

// Do http request
func (s *Session) Do(method, url, contentType string, body io.Reader) (*Response, error) {
	return s.DoRequest(context.Background(), Method(method), URL(url), Header("Content-Type", contentType), Body(body))
}

// DoWithContext http request
func (s *Session) DoWithContext(ctx context.Context, method, url, contentType string, body io.Reader) (*Response, error) {
	return s.DoRequest(ctx, Method(method), URL(url), Header("Content-Type", contentType), Body(body))
}

// Get send get request
func (s *Session) Get(url string) (*Response, error) {
	return s.DoRequest(context.Background(), Method("GET"), URL(url))
}

// Head send head request
func (s *Session) Head(url string) (*Response, error) {
	return s.DoRequest(context.Background(), Method("HEAD"), URL(url))
}

// GetWithContext http request
func (s *Session) GetWithContext(ctx context.Context, url string) (*Response, error) {
	return s.DoRequest(ctx, Method("GET"), URL(url))
}

// Post send post request
func (s *Session) Post(url, contentType string, body io.Reader) (*Response, error) {
	return s.Do("POST", url, contentType, body)
}

// PostWithContext send post request
func (s *Session) PostWithContext(ctx context.Context, url, contentType string, body io.Reader) (*Response, error) {
	return s.DoWithContext(ctx, "POST", url, contentType, body)
}

// PostForm post form request
func (s *Session) PostForm(url string, data url.Values) (*Response, error) {
	return s.Post(url, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}

// PostFormWithContext post form request
func (s *Session) PostFormWithContext(ctx context.Context, url string, data url.Values) (*Response, error) {
	return s.PostWithContext(ctx, url, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}

// Put send put request
func (s *Session) Put(url, contentType string, body io.Reader) (*Response, error) {
	return s.Do("PUT", url, contentType, body)
}

// PutWithContext send put request
func (s *Session) PutWithContext(ctx context.Context, url, contentType string, body io.Reader) (*Response, error) {
	return s.DoWithContext(ctx, "PUT", url, contentType, body)
}

// Delete send delete request
func (s *Session) Delete(url, contentType string, body io.Reader) (resp *Response, err error) {
	return s.Do("DELETE", url, contentType, body)
}

// DeleteWithContext send delete request
func (s *Session) DeleteWithContext(ctx context.Context, url, contentType string, body io.Reader) (*Response, error) {
	return s.DoWithContext(ctx, "DELETE", url, contentType, body)
}

// Upload upload file
func (s *Session) Upload(url, file string) (*Response, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return s.Post(url, "binary/octet-stream", f)
}

// Uploadmultipart upload with multipart form
func (s *Session) Uploadmultipart(url, file string, fields map[string]string) (*Response, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fw, err := writer.CreateFormFile("file", fields["filename"])
	if err != nil {
		return nil, fmt.Errorf("CreateFormFile %v", err)
	}

	_, err = io.Copy(fw, f)
	if err != nil {
		return nil, fmt.Errorf("copying fileWriter %v", err)
	}
	for k, v := range fields {
		if err = writer.WriteField(k, v); err != nil {
			return nil, err
		}
	}

	err = writer.Close() // close writer before POST request
	if err != nil {
		return nil, fmt.Errorf("writerClose: %v", err)
	}

	return s.Post(url, writer.FormDataContentType(), body)
}
