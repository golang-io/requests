package requests

import (
	"fmt"
	"github.com/go-chi/chi/middleware"
	"io"
	"net/http"
	"testing"
)

func Test_NewServer(t *testing.T) {
	s := NewServer(URL("0.0.0.0:9099"),
		Use(WarpHttpHandler(middleware.Logger)),
		//RequestEach(func(ctx context.Context, req *http.Request) error {
		//	//fmt.Println("request each inject", req.URL.Path)
		//	//if req.URL.Path == "/12345" {
		//	//	return errors.New("request each inject")
		//	//}
		//	return nil
		//}),
		Use(func(fn http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				fn(w, r)
			}

		}),
	)
	//s.Path("", func(w http.ResponseWriter, r *http.Request) {
	//	fmt.Fprintf(w, "1234!!!")
	//})
	s.Path("/echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, r.Body)
	})
	s.Path("/ping", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "pong\n")
	}, Use(func(f http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("path use", r.Body)
			f(w, r)
		}
	}))
	s.Run()
}
