package requests_test

import (
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
