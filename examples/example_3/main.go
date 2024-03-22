package main

import (
	"context"
	"github.com/golang-io/requests"
	"log"
)

func main() {
	sess := requests.New(requests.URL("http://httpbin.org/post"))
	resp, err := sess.DoRequest(context.TODO(), requests.MethodPost, requests.Body("Hello world"))
	log.Printf("resp=%s, err=%v", resp.Content, err)
}
