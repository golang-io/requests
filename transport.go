package requests

import (
	"context"
	"fmt"
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
				f(context.Background(), StatLoad(resp))
			}()
			resp.Response, resp.Err = fn(req)
			return resp.Response, resp.Err
		}
	}
}

func verbose(v int, mLimit ...int) func(fn HttpRoundTripFunc) HttpRoundTripFunc {
	max := 10240
	if len(mLimit) != 0 {
		max = mLimit[0]
	}
	return func(fn HttpRoundTripFunc) HttpRoundTripFunc {
		return func(req *http.Request) (*http.Response, error) {
			ctx := httptrace.WithClientTrace(req.Context(), trace)
			req2 := req.WithContext(ctx)
			reqLog, err := httputil.DumpRequestOut(req2, true)
			if err != nil {
				Log("! request error: %w", err)
				return nil, err
			}
			resp, err := fn(req)
			if v >= 2 {
				Log(show("> ", reqLog, max))
			}
			if err != nil {
				return nil, err
			}

			respLog, err := httputil.DumpResponse(resp, v > 3)
			if err != nil {
				return nil, err
			}
			if v > 3 {
				Log(show("< ", respLog, max))
			} else {
				Log("* resp.body is skipped")
			}
			Log("* ")
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
