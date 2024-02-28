package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"time"
)

func main() {
	var addr string
	flag.StringVar(&addr, "http", "0.0.0.0:8080", "监听端口")
	flag.Parse()

	server := &http.Server{
		Addr:    addr,
		Handler: &proxyHandler{},
	}
	_, _ = fmt.Fprintf(os.Stdout, "http serve[%s]...\n", addr)
	if err := server.ListenAndServe(); err != nil {
		_, _ = fmt.Println(err)
	}
}

type proxyHandler struct{}

func (p *proxyHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	status, now := http.StatusBadGateway, time.Now()
	defer func() {
		_, _ = fmt.Fprintf(os.Stdout, "%-18s -> %s [%d] cost: %.2fs\n", r.RemoteAddr, r.RequestURI, status, time.Since(now).Seconds())
	}()
	proxy := &httputil.ReverseProxy{}

	proxy.Director = func(req *http.Request) {}
	proxy.ModifyResponse = func(resp *http.Response) error {
		status = resp.StatusCode
		return nil
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		_, _ = fmt.Fprintf(os.Stdout, "http: proxy error: %v\n", err)
		rw.WriteHeader(http.StatusBadGateway)
	}
	proxy.ServeHTTP(rw, r)
}
