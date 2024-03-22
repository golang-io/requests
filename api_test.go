package requests_test

import (
	"github.com/golang-io/requests"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

var ss = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	_, _ = io.Copy(w, r.Body)
}))

func TestMain(m *testing.M) {

	os.Exit(m.Run())

}

func TestGet(t *testing.T) {
	resp, err := requests.Get(ss.URL)
	stat := requests.StatLoad(&requests.Response{Response: resp, Err: err})
	t.Logf("%s", stat)
}
