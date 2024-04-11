package requests_test

import (
	"context"
	"github.com/golang-io/requests"
	"net/http"
	"testing"
)

func Test_Setup(t *testing.T) {

	sess := requests.New(
		requests.RequestEach(func(ctx context.Context, r *http.Request) error {
			t.Logf("session.RequestEach start")
			defer t.Logf("session.RequestEach defer end")
			t.Logf("session.RequestEach end")
			return nil
		}),
		requests.ResponseEach(func(ctx context.Context, r *http.Response) error {
			t.Logf("session.ResponseEach start")
			defer t.Logf("session.ResponseEach defer end")
			t.Logf("session.ResponseEach end")
			return nil
		}),
		requests.Setup(
			func(fn requests.HttpRoundTripFunc) requests.HttpRoundTripFunc {
				return func(req *http.Request) (*http.Response, error) {
					t.Logf("session.Setup start1")
					defer t.Logf("session.Setup defer end1")
					resp, err := fn(req)
					t.Logf("session.Setup end1")
					return resp, err
				}
			},
			func(fn requests.HttpRoundTripFunc) requests.HttpRoundTripFunc {
				return func(req *http.Request) (*http.Response, error) {
					t.Logf("session.Setup start2")
					defer t.Logf("session.Setup defer end2")
					resp, err := fn(req)
					t.Logf("session.Setup end2")
					return resp, err
				}
			},
		),
	)

	resp, err := sess.DoRequest(
		context.Background(), requests.URL(ss.URL), requests.Body(`{"Hello":"World"}`),
		//requests.Logf(requests.LogS), requests.TraceLv(4),
		requests.RequestEach(func(ctx context.Context, r *http.Request) error {
			t.Logf("doRequest.RequestEach start")
			defer t.Logf("doRequest.RequestEach defer end")
			t.Logf("doRequest.RequestEach end")
			return nil
		}),
		requests.ResponseEach(func(ctx context.Context, r *http.Response) error {
			t.Logf("doRequest.ResponseEach start")
			defer t.Logf("doRequest.ResponseEach defer end")
			t.Logf("doRequest.ResponseEach end")
			return nil
		}),
		requests.Setup(func(fn requests.HttpRoundTripFunc) requests.HttpRoundTripFunc {
			return func(req *http.Request) (*http.Response, error) {
				t.Logf("doRequest.Setup start")
				defer t.Logf("doRequest.Setup defer end")
				resp, err := fn(req)
				t.Logf("doRequest.Setup end")
				return resp, err
			}
		}),
	)
	t.Logf("resp=%s, err=%v", resp.Content.String(), err)
}

func TestRetry(t *testing.T) {
	s := requests.New(requests.Setup(
		requests.Retry(3, requests.RetryHandle),
	), requests.Logf(requests.LogS))
	resp, err := s.DoRequest(context.Background(), requests.URL(""), requests.Body("12345"))
	t.Logf("%s, err=%v", resp.Content.String(), err)
}
