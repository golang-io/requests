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

var OPTIONS = func(next http.Handler) http.Handler {
	fmt.Println("OPTIONS 1")

	f := func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("OPTIONS 2")
		if r.Method == http.MethodOptions {
			http.Error(w, "OPTIONS TEST", http.StatusBadRequest)
		} else {
			next.ServeHTTP(w, r)
		}
		fmt.Println("OPTIONS 3")

	}
	return http.HandlerFunc(f)
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
	s := requests.NewServer(requests.URL("0.0.0.0:9099"),
		requests.Use(RequestLogger(), OPTIONS), // 	"github.com/go-chi/chi/middleware"
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
	s.Route("/echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, r.Body)
	})
	s.Route("/ping", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "pong\n")
	}, requests.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("path use", r.Body)
			next.ServeHTTP(w, r)
		})
	}),
	)

	go func() {
		s.Run(context.Background())
	}()
	time.Sleep(1 * time.Second)
	sess := requests.New(requests.URL("http://127.0.0.1:9099"))
	sess.DoRequest(context.Background(),
		requests.Path("/echo"), requests.Body("12345"),
		requests.Logf(requests.LogS), requests.Method("OPTIONS"),
	)
	//sess.DoRequest(context.Background(), Path("/echo"), Body("12345"), Logf(LogS), Method("GET"))

	//sess.DoRequest(context.Background(), Path("/ping"), Logf(LogS))
	//sess.DoRequest(context.Background(), Path("/1234"), Logf(LogS))

}
