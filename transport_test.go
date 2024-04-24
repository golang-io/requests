package requests_test

import (
	"context"
	"github.com/golang-io/requests"
	"net/http"
	"testing"
)

func Test_Setup(t *testing.T) {

	var setup = func(stage, step string) func(next http.RoundTripper) http.RoundTripper {
		return func(next http.RoundTripper) http.RoundTripper {
			return requests.RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				t.Logf("client stage=%s, step=%s start", stage, step)
				resp, err := next.RoundTrip(req)
				t.Logf("client stage=%s, step=%s end", stage, step)
				return resp, err
			})
		}
	}

	sess := requests.New(
		requests.Setup(setup("session", "step1"), setup("session", "step2")),
	)

	resp, err := sess.DoRequest(
		context.Background(), requests.URL(ss.URL), requests.Body(`{"Hello":"World"}`),
		requests.Setup(setup("request", "step1"), setup("request", "step2")),
	)
	t.Logf("resp=%s, err=%v", resp.Content.String(), err)
}
