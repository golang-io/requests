package requests

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"
)

func TestSession_Do(t *testing.T) {
	sock := "unix:///tmp/requests.sock"
	u, err := url.Parse(sock)
	if err != nil {
		t.Error(err)
	}
	t.Log(u.String(), u.Host, u.Path)
	os.Remove("/tmp/requests.sock")
	l, err := net.Listen("unix", "/tmp/requests.sock")
	if err != nil {
		t.Error(err)
		return
	}

	s := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(os.Stdout, "url=%s, path=%s\n", r.URL.String(), r.URL.Path)
			io.Copy(w, r.Body)
		}),
	}
	defer s.Shutdown(context.Background())
	go func() {
		s.Serve(l)
	}()

	sess := New(URL(sock))
	sess.DoRequest(context.Background(), URL("http://path?k=v"), Body("12345"), MethodPost, Logf(LogS))

}
