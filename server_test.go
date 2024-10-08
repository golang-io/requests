package requests_test

import (
	"context"
	"fmt"
	"github.com/golang-io/requests"
	"io"
	"log"
	"net/http"
	"testing"
	"time"
)

// LogS supply default handle Stat, print to stdout.
func LogS(_ context.Context, stat *requests.Stat) {
	_, _ = fmt.Printf("%s\n", stat)
}

type h struct{}

func (h) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("handle ok"))
}
func Test_Server(t *testing.T) {

	mux := requests.NewServeMux()
	mux.Pprof()
	s := requests.NewServer(context.Background(), mux, requests.URL("http://127.0.0.1:6066"))
	s.OnStartup(func(s *http.Server) { fmt.Println("http serve") })
	mux.Handle("/handle", h{})
	mux.HandleFunc("/handler", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("handler ok"))
	})
	mux.Route("/route_handler", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("route_handler ok"))
	})
	mux.Route("/route_handle", h{})
	mux.Route("/route_func", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("route_func ok"))
	}))

	go s.ListenAndServe()
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

	r := requests.NewServeMux(
		requests.Use(use("step1"), use("step2")),
		requests.Logf(func(ctx context.Context, stat *requests.Stat) {
			log.Printf("%s", stat.Print())
		}),
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s := requests.NewServer(ctx, r, requests.URL("http://0.0.0.0:9099"))
	s.OnShutdown(func(s *http.Server) {
		t.Logf("http: %s shutdown...", s.Addr)
	})
	r.Pprof()
	go s.ListenAndServe()
	time.Sleep(1 * time.Second)
	sess := requests.New(requests.URL("http://127.0.0.1:9099"))
	_, _ = sess.DoRequest(context.Background(), requests.Path("/echo"), requests.Body("12345"), requests.Logf(LogS), requests.Method("OPTIONS"))
	_, _ = sess.DoRequest(context.Background(), requests.Path("/echo"), requests.Body("12345"), requests.Logf(LogS), requests.Method("GET"))
	//sess.DoRequest(context.Background(), Path("/ping"), Logf(LogS))
	//sess.DoRequest(context.Background(), Path("/1234"), Logf(LogS))

	select {
	case <-ctx.Done():
	}

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
