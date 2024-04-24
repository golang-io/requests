package requests_test

import (
	"context"
	"fmt"
	"github.com/golang-io/requests"
	"io"
	"net/http"
	"testing"
	"time"
)

var STEP2 = func(next http.Handler) http.Handler {
	fmt.Println("STEP2 init")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("STEP2 start")
		next.ServeHTTP(w, r)
		fmt.Println("STEP2 end")
	})
}

func Test_Use(t *testing.T) {

	var use = func(name string) func(next http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Logf("server %s start", name)
				defer func() {
					t.Logf("server %s defer end", name)
				}()
				next.ServeHTTP(w, r)
				t.Logf("server %s end", name)
			})
		}
	}

	r := requests.NewServeMux(requests.URL("0.0.0.0:9099"),
		requests.Use(use("step1"), use("step2")),
	)

	r.Route("/echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, r.Body)
	})
	r.Route("/ping",
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprintf(w, "pong\n")
		},
		requests.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Println("path use", r.Body)
				next.ServeHTTP(w, r)
			})
		}),
	)

	r.OnShutdown(func(s *http.Server) {
		t.Logf("http %s onshutdown...", s.Addr)
	})
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		requests.ListenAndServe(ctx, r)
	}()
	time.Sleep(1 * time.Second)
	sess := requests.New(requests.URL("http://127.0.0.1:9099"))
	_, _ = sess.DoRequest(context.Background(), requests.Path("/echo"), requests.Body("12345"), requests.Logf(requests.LogS), requests.Method("OPTIONS"))
	_, _ = sess.DoRequest(context.Background(), requests.Path("/echo"), requests.Body("12345"), requests.Logf(requests.LogS), requests.Method("GET"))
	cancel()
	time.Sleep(3 * time.Second)
	//sess.DoRequest(context.Background(), Path("/ping"), Logf(LogS))
	//sess.DoRequest(context.Background(), Path("/1234"), Logf(LogS))

}

var f = func(w http.ResponseWriter, r *http.Request) {
	_, _ = fmt.Fprintf(w, "pong\n")
}

func Test_Node(t *testing.T) {
	r := requests.NewNode("/", nil)
	r.Add("/abc/def/ghi", f)
	r.Add("/abc/def/xyz", f)
	r.Add("/1/2/3", f)
	r.Add("/abc/def", f)
	r.Add("/abc/def/", f)
	r.Add("/abc/def/", f)
	r.Add("/", f)
	r.Print()
	//go requests.ListenAndServe(context.Background(), r, requests.URL("0.0.0.0:1234"))
	//fmt.Println(r)
}
