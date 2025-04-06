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

// newTransport creates a new Transport instance.
// It configures connection pool, timeout settings, TLS, and other parameters.
func newTransport(opts ...Option) *http.Transport {
	options := newOptions(opts)
	return &http.Transport{
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
			return socket(ctx, options.LocalAddr, network, addr, 10*time.Second)
		},

		// Connection pool configuration
		MaxIdleConns:        options.MaxConns,  // Maximum number of idle connections
		MaxIdleConnsPerHost: options.MaxConns,  // Maximum number of idle connections per host
		IdleConnTimeout:     120 * time.Second, // Idle connection timeout

		// Connection behavior configuration
		DisableCompression: true,  // Disable compression
		DisableKeepAlives:  false, // Enable Keep-Alive

		// TLS configuration
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !options.Verify, // Whether to verify server certificates
		},
	}
}
