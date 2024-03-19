# request

**Requests** is a simle, yet elegant, HTTP library. 

Golang HTTP Requests for Humans™ ✨🍰✨

### Usage

* Basic Usage

```(golang)
requests.Get("http://httpbin.org/get")
requests.Post(
    "http://httpbin.org/post", 
    "application/json", 
    strings.NewReader(`{"a": "b"}`),
)
```

* Advanced Usage

```(golang)
package main

import (
    "log"
    "fmt
    "github.com/golang-io/requests"
)

func main() {
    // 创建session, 全局配置, 会追加到使用这个sess的所有请求中
    sess := requests.New(requests.Auth("user", "123456"))   
    resp, err := sess.DoRequest(nil,
        requests.Method("POST"),
        requests.URL("http://httpbin.org/post"),
        requests.Params(map[string]interface{}{
            "a": "b",
            "c": 3,
            "d": []int{1, 2, 3},
        }),
        requests.Body(`{"body":"QWER"}`),
        requests.Retry(3),
        requests.Header("hello", "world"),
    )   // 创建一个POST请求
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(resp.Text())
}
```

## Supported Features & Best–Practices
* Safe Close `resp.Body`
* Only depends on standard library
* Streaming Downloads
* Chunked HTTP Requests
* Keep-Alive & Connection Pooling
* Sessions with Cookie Persistence
* Basic & Digest Authentication
* Implement http.RoundTripper fully compatible `net/http`


## API Reference and User Guide available on [Read the Docs](https://pkg.go.dev/github.com/golang-io/requests)