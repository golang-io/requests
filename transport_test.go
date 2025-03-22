package requests

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func Test_Setup(t *testing.T) {
	var setups []string
	var setup = func(stage, step string) func(next http.RoundTripper) http.RoundTripper {
		return func(next http.RoundTripper) http.RoundTripper {
			return RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				setups = append(setups, strings.Join([]string{stage, step, "start"}, "-"))
				resp, err := next.RoundTrip(req)
				setups = append(setups, strings.Join([]string{stage, step, "end"}, "-"))
				return resp, err
			})
		}
	}

	var wants = []string{
		"session-step1-start", "session-step2-start", "request-step1-start", "request-step2-start",
		"request-step2-end", "request-step1-end", "session-step2-end", "session-step1-end",
	}

	var ss = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, r.Body)
	}))
	sess := New(Setup(setup("session", "step1"), setup("session", "step2")))

	for m := 0; m < 4; m++ {
		setups = setups[:0]
		resp, err := sess.DoRequest(context.Background(), URL(ss.URL), Body(`{"Hello":"World"}`), Setup(setup("request", "step1"), setup("request", "step2")))
		t.Logf("resp=%s, err=%v", resp.Content.String(), err)
		if len(setups) != len(wants) {
			t.Error("len(setups)!= len(setups)")
			return
		}
		for i := 0; i < len(setups); i++ {
			if setups[i] != wants[i] {
				t.Errorf("setups=%v, wants=%v", setups[i], wants[i])
				return
			}
			t.Logf("setups=%v, wants=%v", setups[i], wants[i])
		}
	}

}
