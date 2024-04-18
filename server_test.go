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

var STEP1 = func(next http.Handler) http.Handler {
	fmt.Println("STEP1 1")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("STEP1 2")
		next.ServeHTTP(w, r)
		fmt.Println("STEP1 3")
	})
}

var STEP2 = func(next http.Handler) http.Handler {
	fmt.Println("STEP2 1")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("STEP2 2")
		next.ServeHTTP(w, r)
		fmt.Println("STEP2 3")
	})
}

// RequestLogger returns a logger handler using a custom LogFormatter.
func RequestLogger() func(next http.Handler) http.Handler {
	fmt.Println("RequestLogger 1")

	return func(next http.Handler) http.Handler {
		fmt.Println("RequestLogger 2")

		fn := func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("RequestLogger 3")

			defer func() {
				fmt.Println("RequestLogger 1")
			}()

			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

func Test_NewServer(t *testing.T) {
	r := requests.NewServeMux(requests.URL("0.0.0.0:9099"),
		requests.Use(RequestLogger(), STEP1, STEP2), // 	"github.com/go-chi/chi/middleware"
		//RequestEach(func(ctx context.Context, req *http.Request) error {
		//	//fmt.Println("request each inject", req.URL.Path)
		//	//if req.URL.Path == "/12345" {
		//	//	return errors.New("request each inject")
		//	//}
		//	return nil
		//}),
	)
	//s.Path("", func(w http.ResponseWriter, r *http.Request) {
	//	fmt.Fprintf(w, "1234!!!")
	//})
	r.Route("/echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, r.Body)
	})
	r.Route("/ping", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "pong\n")
	}, requests.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("path use", r.Body)
			next.ServeHTTP(w, r)
		})
	}),
	)

	go func() {
		requests.ListenAndServe(context.Background(), r)
	}()
	time.Sleep(1 * time.Second)
	sess := requests.New(requests.URL("http://127.0.0.1:9099"))
	_, _ = sess.DoRequest(context.Background(), requests.Path("/echo"), requests.Body("12345"), requests.Logf(requests.LogS), requests.Method("OPTIONS"))
	_, _ = sess.DoRequest(context.Background(), requests.Path("/echo"), requests.Body("12345"), requests.Logf(requests.LogS), requests.Method("GET"))

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
