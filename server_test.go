package requests

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
)

func Test_NewServer(t *testing.T) {
	s := NewServer(URL("0.0.0.0:9099"),
		//Use(WarpHttpHandler(middleware.Logger)), // 	"github.com/go-chi/chi/middleware"
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
	s.Route("/echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, r.Body)
	})
	s.Route("/ping", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "pong\n")
	}, Use(func(f http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("path use", r.Body)
			f(w, r)
		}
	}))

	go func() {
		s.Run(context.Background())
	}()
	sess := New(URL("http://127.0.0.1:9099"))
	sess.DoRequest(context.Background(), Path("/echo"), Body("12345"), Logf(LogS))
	sess.DoRequest(context.Background(), Path("/ping"), Logf(LogS))
	sess.DoRequest(context.Background(), Path("/1234"), Logf(LogS))

}
