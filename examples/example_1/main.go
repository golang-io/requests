package main

import (
	"context"
	"github.com/golang-io/requests"
)

func main() {
	sess := requests.New(requests.URL("https://httpbin.org/uuid"), requests.TraceLv(4))
	_, _ = sess.DoRequest(context.TODO())
}
