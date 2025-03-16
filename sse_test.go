package requests_test

import (
	"bytes"
	"context"
	"fmt"
	"github.com/golang-io/requests"
	"log"
	"net/http"
	"testing"
	"time"
)

func Test_SSE(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := requests.NewServeMux(requests.Logf(requests.LogS))
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
	}, requests.Use(requests.SSE()))
	s := requests.NewServer(ctx, r, requests.URL("http://0.0.0.0:1234"))
	go s.ListenAndServe()
	time.Sleep(1 * time.Second)
	c := requests.New(requests.Logf(requests.LogS))
	resp, err := c.DoRequest(ctx, requests.URL("http://0.0.0.0:1234/sse"),
		requests.Stream(func(i int64, b []byte) error {
			return SSE(i, b, func(b []byte) error {
				_, _ = fmt.Printf("%s\n", b)
				return nil
			})

		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf("response=%s", resp.Content.String())

	log.Printf("----------------")
	resp, err = c.DoRequest(ctx, requests.URL("http://0.0.0.0:1234/123"), requests.Body(`{"a":"b"}`))
	if err != nil {
		t.Fatal(err)
	}
	log.Printf("response=%s", resp.Content.String())
	cancel()

	time.Sleep(1 * time.Second)
}

func SSE(i int64, b []byte, f func([]byte) error) error {
	name, value, _ := bytes.Cut(bytes.TrimRight(b, "\n"), []byte(":"))
	switch string(name) {
	case "data":
		return f(value)
	default:
		return nil
	}
}
