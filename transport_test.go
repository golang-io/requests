package requests

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test_Setup(t *testing.T) {

	var setup = func(stage, step string) func(next http.RoundTripper) http.RoundTripper {
		return func(next http.RoundTripper) http.RoundTripper {
			return RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				t.Logf("client stage=%s, step=%s start", stage, step)
				resp, err := next.RoundTrip(req)
				t.Logf("client stage=%s, step=%s end", stage, step)
				return resp, err
			})
		}
	}
	var ss = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, r.Body)
	}))
	sess := New(
		Setup(setup("session", "step1"), setup("session", "step2")),
	)

	resp, err := sess.DoRequest(
		context.Background(), URL(ss.URL), Body(`{"Hello":"World"}`),
		Setup(setup("request", "step1"), setup("request", "step2")),
	)
	t.Logf("resp=%s, err=%v", resp.Content.String(), err)
}
