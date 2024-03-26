package main

import (
	"fmt"
	"github.com/go-chi/chi/middleware"
	"github.com/golang-io/requests"
	"io"
	"net/http"
)

func main() {
	s := requests.NewServer(
		requests.URL("0.0.0.0:1234"),
		requests.Use(requests.WarpHttpHandler(middleware.Logger)), //
		//RequestEach(func(ctx context.Context, req *http.Request) error {
		//	//fmt.Println("request each inject", req.URL.Path)
		//	//if req.URL.Path == "/12345" {
		//	//	return errors.New("request each inject")
		//	//}
		//	return nil
		//}),
		requests.Use(func(fn http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				fn(w, r)
			}

		}),
	)
	s.Path("/echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, r.Body)
	})
	s.Path("/ping", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "pong\n")
	}, requests.Use(func(f http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			f(w, r)
		}
	}))
	err := s.Run()
	fmt.Println(err)
}
