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

// WarpRoundTripper wraps an http.RoundTripper instance.
// This function returns a new decorator function that adds additional functionality to an existing RoundTripper.
func WarpRoundTripper(next http.RoundTripper) func(http.RoundTripper) http.RoundTripper {
	return func(http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return next.RoundTrip(r)
		})
	}
}

// RoundTripperFunc is a functional implementation of the http.RoundTripper interface.
// It allows converting regular functions to the RoundTripper interface, facilitating functional extensions.
type RoundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip implements the http.RoundTripper interface.
// It directly calls the underlying function to complete the request sending and response receiving.
func (fn RoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

// printRoundTripper creates a middleware for printing HTTP client request and response information.
// Parameter f is a callback function for processing request statistics.
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

// printHandler creates a middleware for printing HTTP server request and response information.
// It records the request processing time and related statistics.
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

// Transport is a custom HTTP transport layer implementation.
// It wraps the standard library's http.Transport and adds additional configuration options.
type Transport struct {
	opts []Option          // Stores transport layer configuration options
	*http.Transport       // Embeds the standard library's Transport
}

// newTransport creates a new Transport instance.
// It configures connection pool, timeout settings, TLS, and other parameters.
func newTransport(opts ...Option) *Transport {
	options := newOptions(opts)
	return &Transport{
		opts: opts,
		Transport: &http.Transport{
			// Proxy sets the proxy function
			Proxy: options.Proxy,
			
			// DialContext customizes connection creation logic
			// Supports Unix domain sockets and TCP connections
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				// Handle Unix domain socket connections
				if strings.HasPrefix(options.URL, "unix://") {
					u, err := url.Parse(options.URL)
					if err != nil {
						return nil, err
					}
					network, addr = u.Scheme, u.Path
				}
				
				// Configure dialer parameters
				dialer := net.Dialer{
					Timeout:   10 * time.Second,  // TCP connection timeout
					KeepAlive: 60 * time.Second,  // TCP keepalive interval
					LocalAddr: options.LocalAddr, // Local address binding
					Resolver: &net.Resolver{      // DNS resolver configuration
						PreferGo:     true,       // Prefer Go's DNS resolver
						StrictErrors: false,      // Tolerate DNS resolution errors
					},
				}
				return dialer.DialContext(ctx, network, addr)
			},
			
			// Connection pool configuration
			MaxIdleConns:        options.MaxConns,     // Maximum number of idle connections
			MaxIdleConnsPerHost: options.MaxConns,     // Maximum number of idle connections per host
			IdleConnTimeout:     120 * time.Second,    // Idle connection timeout
			
			// Connection behavior configuration
			DisableCompression: true,                  // Disable compression
			DisableKeepAlives:  false,                 // Enable Keep-Alive
			
			// TLS configuration
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: !options.Verify,   // Whether to verify server certificates
			},
		},
	}
}

// RoundTrip implements the RoundTripper interface.
// It processes requests by calling the RoundTripper method.
func (t *Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	return t.RoundTripper().RoundTrip(r)
}

// RoundTripper returns a configured http.RoundTripper.
// It applies all registered middleware in reverse order.
func (t *Transport) RoundTripper(opts ...Option) http.RoundTripper {
	return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		options := newOptions(t.opts, opts...)
		if options.Transport == nil {
			options.Transport = t.Transport
		}
		// Apply middleware in reverse order
		for i := len(options.HttpRoundTripper) - 1; i >= 0; i-- {
			options.Transport = options.HttpRoundTripper[i](options.Transport)
		}
		return options.Transport.RoundTrip(r)
	})
}

// Redirect creates a middleware for handling HTTP redirects.
// It handles 301 (Moved Permanently) and 302 (Found) status codes.
func Redirect(next http.RoundTripper) http.RoundTripper {
	return RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		response, err := next.RoundTrip(req)
		if err != nil {
			return response, err
		}
		// Check if redirection is needed
		if response.StatusCode != http.StatusMovedPermanently && response.StatusCode != http.StatusFound {
			return response, err
		}
		// Create redirect request
		if req, err = NewRequestWithContext(req.Context(), Options{
			Method: req.Method,
			URL:    response.Header.Get("Location"),
			Header: req.Header,
			body:   req.Body,
		}); err != nil {
			return response, err
		}
		// Execute redirect request
		return next.RoundTrip(req)
	})
}
