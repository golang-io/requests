package requests

import (
	"context"
	"net"
	"net/url"
	"time"
)

// Socket socket ..
func socket(ctx context.Context, src net.Addr, network, address string, timeout time.Duration) (net.Conn, error) {
	dialer := net.Dialer{
		Timeout:   timeout,          // TCP connection timeout
		KeepAlive: 60 * time.Second, // TCP keepalive interval
		LocalAddr: src,              // Local address binding
		Resolver: &net.Resolver{ // DNS resolver configuration
			PreferGo:     true,  // Prefer Go's DNS resolver
			StrictErrors: false, // Tolerate DNS resolution errors
		},
	}
	return dialer.DialContext(ctx, network, address)
}

func Socket(ctx context.Context, opts ...Option) (net.Conn, error) {
	options := newOptions(opts)
	u, err := url.Parse(options.URL)
	if err != nil {
		return nil, err
	}
	return socket(ctx, options.LocalAddr, u.Scheme, u.Host, options.Timeout)
}
