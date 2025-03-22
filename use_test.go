package requests

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func Test_SSE(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := NewServeMux(Logf(LogS))
	r.Route("/123", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("hello world"))
	})
	r.Route("/sse", func(w http.ResponseWriter, r *http.Request) {

		for i := 0; i < 3; i++ {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(1 * time.Second):
				w.Write([]byte(fmt.Sprintf(`{"a":"12345\n", "b": %d}`, i)))
			}
		}
	}, Use(SSE()))
	s := NewServer(ctx, r, URL("http://0.0.0.0:1234"))
	go s.ListenAndServe()
	time.Sleep(1 * time.Second)
	c := New(Logf(LogS))
	_, err := c.DoRequest(ctx, URL("http://0.0.0.0:1234/sse"),
		Stream(func(i int64, b []byte) error {

			t.Logf("i=%d, b=%s", i, b)
			return nil

		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.DoRequest(ctx, URL("http://0.0.0.0:1234/123"), Body(`{"a":"b"}`))
	if err != nil {
		t.Fatal(err)
	}

	cancel()

	time.Sleep(1 * time.Second)
}

func SSERound(i int64, b []byte, f func([]byte) error) error {
	name, value, _ := bytes.Cut(bytes.TrimRight(b, "\n"), []byte(":"))
	switch string(name) {
	case "data":
		return f(value)
	default:
		return nil
	}
}
