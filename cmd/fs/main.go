package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-io/requests"
	"io"
	"log"
	"net/http"
	"os"
)

func Token(token string) requests.Option {
	return requests.RequestEach(func(ctx context.Context, r *http.Request) error {
		if token == "" || r.Header.Get("Token") != token {
			return fmt.Errorf("token header is must")
		}
		return nil
	})
}

func Method(method string) requests.Option {
	return requests.RequestEach(func(ctx context.Context, r *http.Request) error {
		if method != "" && r.Method != method {
			return fmt.Errorf("%d", http.StatusMethodNotAllowed)
		}
		return nil
	})
}

func Echo(w http.ResponseWriter, r *http.Request) { _, _ = io.Copy(w, r.Body) }

func RequestLog(output string) func(http.Handler) http.Handler {
	out := os.Stdout
	if output != "stdout" {
		var err error
		if out, err = os.OpenFile(output, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644); err != nil {
			out = os.Stdout
			fmt.Println(err)
		}
	}
	return middleware.RequestLogger(&middleware.DefaultLogFormatter{Logger: log.New(out, "", log.LstdFlags)})
}

// curl http://127.0.0.1:8080 -F '123=@xxx.json' -F '456=@jjj.json' -vvvv
func main() {
	var token = flag.String("token", "byFjRL3cr4v656AojKjW", "上传使用的TOKEN")
	var prefix = flag.String("prefix", "/tmp", "上传文件的前缀路径")
	var listen = flag.String("listen", "0.0.0.0:8080", "监听端口")
	var output = flag.String("output", "stdout", "日志输出")
	flag.Parse()
	r := requests.NewServeMux(requests.URL(*listen), requests.Use(RequestLog(*output), middleware.Recoverer))
	r.Route("/echo", Echo)
	r.Route("/_upload", requests.UploadHandlerFunc(*prefix), Token(*token), Method("POST"))
	// 这里path和prefix都必须以/结尾，否则的话只能处理/backup/a/b/，处理不了/backup/a/b的场景
	r.Route("/backup/", requests.ServeFS("/backup/", "/backup"), Method("GET"))
	r.Pprof()
	r.Route("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/backup/", http.StatusMovedPermanently)
	})
	err := requests.ListenAndServe(context.Background(), r)
	fmt.Println(err)
}
