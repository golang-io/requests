package requests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

// Session httpclient session
// Clients and Transports are safe for concurrent use by multiple goroutines
// for efficiency should only be created once and re-used.
// so, session is also safe for concurrent use by multiple goroutines.
type Session struct {
	opts      []Option
	transport *http.Transport
	client    *http.Client
}

// New session
func New(opts ...Option) *Session {
	options := newOptions(opts)
	transport := newTransport(opts...)
	client := &http.Client{Timeout: options.Timeout, Transport: transport}
	s := &Session{opts: opts, transport: transport, client: client}
	return s
}

// HTTPClient returns the http.Client that is configured to be used for HTTP requests.
func (s *Session) HTTPClient() *http.Client {
	return s.client
}

// Transport returns *http.Transport.
func (s *Session) Transport() *http.Transport {
	return s.transport
}

// RoundTrip implements the [RoundTripper] interface.
// Like the `http.RoundTripper` interface, the error types returned by RoundTrip are unspecified.
func (s *Session) RoundTrip(r *http.Request) (*http.Response, error) {
	return s.RoundTripper().RoundTrip(r)
}

// Do send a request and  return `http.Response`. DO NOT forget close `resp.Body`.
// transport【http.Transport】-> http.client.Do -> transport.RoundTrip
func (s *Session) Do(ctx context.Context, opts ...Option) (*http.Response, error) {
	options := newOptions(s.opts, opts...)
	req, err := NewRequestWithContext(ctx, options)
	if err != nil {
		return &http.Response{}, fmt.Errorf("newRequest: %w", err)
	}
	return s.RoundTripper(opts...).RoundTrip(req)
}

// DoRequest send a request and return a response, and is safely close `resp.Body`.
func (s *Session) DoRequest(ctx context.Context, opts ...Option) (*Response, error) {
	options, resp := newOptions(s.opts, opts...), newResponse(nil)
	resp.Request, resp.Err = NewRequestWithContext(ctx, options)
	if resp.Err != nil {
		return resp, resp.Err
	}

	resp.Response, resp.Err = s.RoundTripper(opts...).RoundTrip(resp.Request)
	if resp.Err != nil {
		return resp, resp.Err
	}

	if resp.Response == nil {
		resp.Response = &http.Response{Body: http.NoBody}
	} else if resp.Response.Body == nil {
		resp.Response.Body = http.NoBody
	}
	defer resp.Response.Body.Close()
	_, resp.Err = resp.Content.ReadFrom(resp.Response.Body)
	resp.Response.Body = io.NopCloser(bytes.NewReader(resp.Content.Bytes()))
	return resp, resp.Err
}

// RoundTripper returns a configured http.RoundTripper.
// It applies all registered middleware in reverse order.
func (s *Session) RoundTripper(opts ...Option) http.RoundTripper {
	return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		options := newOptions(s.opts, opts...)
		if options.Transport == nil {
			options.Transport = RoundTripperFunc(s.client.Do)
		}
		// Apply middleware in reverse order
		for i := len(options.HttpRoundTripper) - 1; i >= 0; i-- {
			options.Transport = options.HttpRoundTripper[i](options.Transport)
		}
		return options.Transport.RoundTrip(r)
	})
}
