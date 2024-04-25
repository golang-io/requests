package requests

import (
	"context"
	"net/http"
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

// Stream 和Log不能共用
func fprintf(f func(ctx context.Context, stat *Stat)) func(http.RoundTripper) http.RoundTripper {
	resp := newResponse()
	return func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			resp.Request = r
			defer func() {
				f(r.Context(), resp.Stat())
			}()
			resp.Response, resp.Err = next.RoundTrip(r)
			return resp.Response, resp.Err
		})
	}
}
