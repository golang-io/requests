package main

import (
	"context"
	"fmt"
	"github.com/golang-io/requests"
)

func main() {
	sess := requests.New(requests.Proxy("http://127.0.0.1:8080"))
	resp, err := sess.DoRequest(context.Background(), requests.URL("http://baidu.com"), requests.TraceLv(3, 100000))
	fmt.Println(resp, err)
}
