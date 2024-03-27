package requests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
)

// HttpRoundTripFunc is a http.RoundTripper implementation, which is a simple function.
type HttpRoundTripFunc func(req *http.Request) (resp *http.Response, err error)

// RoundTrip implements http.RoundTripper.
func (fn HttpRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

// Stream 和Log不能共用
func fprintf(f func(ctx context.Context, stat *Stat)) func(HttpRoundTripFunc) HttpRoundTripFunc {
	resp := newResponse()
	return func(fn HttpRoundTripFunc) HttpRoundTripFunc {
		return func(req *http.Request) (*http.Response, error) {
			resp.Request = req
			defer func() {
				f(req.Context(), StatLoad(resp))
			}()
			resp.Response, resp.Err = fn(req)
			return resp.Response, resp.Err
		}
	}
}

func verbose(v int, mLimit ...int) func(fn HttpRoundTripFunc) HttpRoundTripFunc {
	return func(fn HttpRoundTripFunc) HttpRoundTripFunc {
		return func(req *http.Request) (*http.Response, error) {
			if v == 0 {
				return fn(req) // fast path
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
			resp, err := fn(req2)
			if v >= 2 {
				Log(show("> ", reqLog, maxLimit))
			}
			if err != nil {
				return nil, err
			}

			if v >= 3 {
				// 答应响应头和响应体长度
				Log("< %s %s", resp.Proto, resp.Status)
				for k, vs := range resp.Header {
					for _, v := range vs {
						Log("< %s: %s", k, v)
					}
				}
			}
			if v >= 4 {
				buf, err := CopyResponseBody(resp)
				if err != nil {
					Log("! response error: %w", err)
					return nil, err
				}
				Log(show("", buf.Bytes(), maxLimit))
			}
			return resp, nil
		}
	}
}

func each(options Options) func(HttpRoundTripFunc) HttpRoundTripFunc {
	return func(fn HttpRoundTripFunc) HttpRoundTripFunc {
		return func(req *http.Request) (*http.Response, error) {
			for _, each := range options.OnRequest {
				if err := each(req.Context(), req); err != nil {
					return &http.Response{}, fmt.Errorf("requestEach: %w", err)
				}
			}
			resp, err := fn(req)
			if err != nil {
				return resp, err
			}
			for _, each := range options.OnResponse {
				if err := each(req.Context(), resp); err != nil {
					return resp, fmt.Errorf("responseEach: %w", err)
				}
			}
			return resp, err
		}
	}
}

func Retry(maxLimit int, check func(*http.Request, *http.Response, error) error) func(HttpRoundTripFunc) HttpRoundTripFunc {
	return func(fn HttpRoundTripFunc) HttpRoundTripFunc {
		return func(req *http.Request) (*http.Response, error) {
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(req.Body)
			var resp *http.Response
			var err error
			for i := 0; i < maxLimit; i++ {
				req2 := req.WithContext(req.Context())
				req2.Body = io.NopCloser(bytes.NewReader(buf.Bytes()))
				resp, err = fn(req2)
				if err = check(req2, resp, err); err == nil {
					return resp, err
				}
			}
			return resp, err
		}
	}
}
