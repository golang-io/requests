package requests

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
	"net/textproto"
)

// ClientTrace is a set of hooks to run at various stages of an outgoing
// HTTP request. Any particular hook may be nil. Functions may be
// called concurrently from different goroutines and some may be called
// after the request has completed or failed.
//
// ClientTrace currently traces a single HTTP request & response
// during a single round trip and has no hooks that span a series
// of redirected requests.
//
// See https://blog.golang.org/http-tracing for more.
var trace = &httptrace.ClientTrace{
	// GetConn is called before a connection is created or
	// retrieved from an idle pool. The hostPort is the
	// "host:port" of the target or proxy. GetConn is called even
	// if there's already an idle cached connection available.
	GetConn: func(hostPort string) {
		Log("* Connect: %v", hostPort)
	},

	// GotConn is called after a successful connection is
	// obtained. There is no hook for failure to obtain a
	// connection; instead, use the error from
	// Transport.RoundTrip.
	GotConn: func(connInfo httptrace.GotConnInfo) {
		Log("* Got Conn: %v -> %v", connInfo.Conn.LocalAddr(), connInfo.Conn.RemoteAddr())
	},
	// PutIdleConn is called when the connection is returned to
	// the idle pool. If err is nil, the connection was
	// successfully returned to the idle pool. If err is non-nil,
	// it describes why not. PutIdleConn is not called if
	// connection reuse is disabled via Transport.DisableKeepAlives.
	// PutIdleConn is called before the caller's Response.Body.Close
	// call returns.
	// For HTTP/2, this hook is not currently used.
	PutIdleConn: func(err error) {},

	// GotFirstResponseByte is called when the first byte of the response
	// headers is available.
	GotFirstResponseByte: func() {},

	// Got100Continue is called if the server replies with a "100
	// Continue" response.
	Got100Continue: func() {},

	// Got1xxResponse is called for each 1xx informational response header
	// returned before the final non-1xx response. Got1xxResponse is called
	// for "100 Continue" responses, even if Got100Continue is also defined.
	// If it returns an error, the client request is aborted with that error value.
	Got1xxResponse: func(code int, header textproto.MIMEHeader) error { return nil },

	// DNSStart is called when a DNS lookup begins.
	DNSStart: func(dnsInfo httptrace.DNSStartInfo) {
		Log("* Resolved Host: %v", dnsInfo.Host)
	},
	// DNSDone is called when a DNS lookup ends.
	DNSDone: func(dnsInfo httptrace.DNSDoneInfo) {
		var ipaddrs []string
		for _, ipaddr := range dnsInfo.Addrs {
			ipaddrs = append(ipaddrs, ipaddr.String())
		}
		Log("* Resolved DNS: %v, Coalesced: %v, err=%v", ipaddrs, dnsInfo.Coalesced, dnsInfo.Err)
	},
	// ConnectStart is called when a new connection's Dial begins.
	// If net.Dialer.DualStack (IPv6 "Happy Eyeballs") support is
	// enabled, this may be called multiple times.
	ConnectStart: func(network, addr string) {
		Log("* Trying ConnectStart %v %v...", network, addr)
	},
	// ConnectDone is called when a new connection's Dial
	// completes. The provided err indicates whether the
	// connection completed successfully.
	// If net.Dialer.DualStack ("Happy Eyeballs") support is
	// enabled, this may be called multiple times.
	ConnectDone: func(network, addr string, err error) {
		Log("* Completed connection: %v %v, err=%v", network, addr, err)
	},
	// TLSHandshakeStart is called when the TLS handshake is started. When
	// connecting to an HTTPS site via an HTTP proxy, the handshake happens
	// after the CONNECT request is processed by the proxy.
	TLSHandshakeStart: func() {},

	// TLSHandshakeDone is called after the TLS handshake with either the
	// successful handshake's connection state, or a non-nil error on handshake
	// failure.
	TLSHandshakeDone: func(state tls.ConnectionState, err error) {
		Log("* SSL HandshakeComplete: %v", state.HandshakeComplete)
	},
	// WroteHeaderField is called after the Transport has written
	// each request header. At the time of this call the values
	// might be buffered and not yet written to the network.
	WroteHeaderField: func(key string, value []string) {},

	// WroteHeaders is called after the Transport has written
	// all request headers.
	WroteHeaders: func() {},

	// Wait100Continue is called if the Request specified
	// "Expect: 100-continue" and the Transport has written the
	// request headers but is waiting for "100 Continue" from the
	// server before writing the request body.
	Wait100Continue: func() {},

	// WroteRequest is called with the result of writing the
	// request and any body. It may be called multiple times
	// in the case of retried requests.
	WroteRequest: func(reqInfo httptrace.WroteRequestInfo) {
		//Log("* WroteRequest, err=%v", reqInfo.Err)
	},
}

// Log print
func Log(format string, v ...any) {
	print(fmt.Sprintf(format+"\n", v...))
}

func show(prompt string, b []byte, mLimit int) string {
	var buf bytes.Buffer
	for _, line := range bytes.Split(b, []byte("\n")) {
		buf.Write([]byte(prompt))
		buf.Write(bytes.Replace(line, []byte("%"), []byte("%%"), -1))
		buf.WriteString("\n")
	}
	str := buf.String()
	if len(str) > mLimit {
		return fmt.Sprintf("%s...[Len=%d, Truncated[%d]]", str[:mLimit], len(str), mLimit)
	}
	return str
}

// Trace trace print
func Trace(mLimit ...int) Option {
	return func(o *Options) {
		o.HttpRoundTripper = append([]func(http.RoundTripper) http.RoundTripper{traceLv(true, mLimit...)}, o.HttpRoundTripper...)
	}
}

func traceLv(used bool, mLimit ...int) func(http.RoundTripper) http.RoundTripper {
	return func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if !used {
				return next.RoundTrip(req) // fast path
			}
			maxLimit := 10240
			if len(mLimit) != 0 {
				maxLimit = mLimit[0]
			}
			ctx := httptrace.WithClientTrace(req.Context(), trace)
			req2 := req.WithContext(ctx)
			reqLog, err := httputil.DumpRequestOut(req2, true)
			if err != nil {
				Log("! request error: %v", err)
				return nil, err
			}
			resp, err := next.RoundTrip(req2)

			Log(show("> ", reqLog, maxLimit))

			if err != nil {
				return nil, err
			}

			// 答应响应头和响应体长度
			Log("< %s %s", resp.Proto, resp.Status)
			for k, vs := range resp.Header {
				for _, v := range vs {
					Log("< %s: %s", k, v)
				}
			}

			buf, r, err := CopyBody(resp.Body)
			if err != nil {
				Log("! response error: %w", err)
				return nil, err
			}
			resp.Body = r
			Log(show("", buf.Bytes(), maxLimit))

			return resp, nil
		})
	}
}
