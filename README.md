# <center>requests</center>
<div style="text-align: center;">
    <div><strong>Requests is a simple, yet elegant, Go HTTP client and server library for Humans™ ✨🍰✨</strong></div>
    <a href="https://pkg.go.dev/github.com/golang-io/requests"><img src="https://pkg.go.dev/badge/github.com/golang-io/requests.svg" alt="Go Reference"></a>
    <a href="https://www.apache.org/licenses/LICENSE-2.0"><img src="https://img.shields.io/badge/license-Apache%20V2-blue.svg" alt="Apache V2 License"></a>
    <a href="https://github.com/golang-io/requests/actions/workflows/go.yml"><img src="https://github.com/golang-io/requests/actions/workflows/go.yml/badge.svg?branch=main" alt="Build status"></a>
    <a href="https://goreportcard.com/report/github.com/golang-io/requests"><img src="https://goreportcard.com/badge/github.com/golang-io/requests" alt="go report"></a>
	<a href="https://sourcegraph.com/github.com/golang-io/requests?badge"><img src="https://sourcegraph.com/github.com/golang-io/requests/-/badge.svg" alt="requests on Sourcegraph"></a>
	<a href="https://codecov.io/gh/golang-io/requests" > <img src="https://codecov.io/gh/golang-io/requests/graph/badge.svg?token=T8MZ92JL1T"/> </a>

</div>
<hr/>

## Overview

Requests is inspired by Python's famous requests library, bringing a more elegant, intuitive approach to HTTP in Go. This library simplifies common HTTP tasks while remaining fully compatible with Go's standard library.

#### API Reference and User Guide available on [Read the Docs](https://pkg.go.dev/github.com/golang-io/requests)

#### Supported Features & Best–Practices
* Safe Close `resp.Body`
* Only depends on standard library
* Streaming Downloads
* Chunked HTTP Requests
* Keep-Alive & Connection Pooling
* Sessions with Cookie Persistence
* Basic & Digest Authentication
* Implement http.RoundTripper fully compatible with `net/http`
* Offer File System to upload or download files easily

## Installation

```shell
go get github.com/golang-io/requests
```

## Quick Start
#### Get Started
```shell
cat github.com/golang-io/examples/example_1/main.go
```

```go
package main

import (
	"context"
	"github.com/golang-io/requests"
)

func main() {
	sess := requests.New(requests.URL("https://httpbin.org/uuid"), requests.TraceLv(4))
	_, _ = sess.DoRequest(context.TODO())
}

```

```shell
$ go run github.com/golang-io/examples/example_1/main.go
* Connect: httpbin.org:80
* Got Conn: <nil> -> <nil>
* Connect: httpbin.org:443
* Resolved Host: httpbin.org
* Resolved DNS: [50.16.63.240 107.21.176.221 3.220.97.10 18.208.241.22], Coalesced: false, err=<nil>
* Trying ConnectStart tcp 50.16.63.240:443...
* Completed connection: tcp 50.16.63.240:443, err=<nil>
* SSL HandshakeComplete: true
* Got Conn: 192.168.255.10:64170 -> 50.16.63.240:443
> GET /uuid HTTP/1.1
> Host: httpbin.org
> User-Agent: Go-http-client/1.1
> Accept-Encoding: gzip
> 
> 

< HTTP/1.1 200 OK
< Content-Length: 53
< Access-Control-Allow-Credentials: true
< Access-Control-Allow-Origin: *
< Connection: keep-alive
< Content-Type: application/json
< Date: Fri, 22 Mar 2024 12:16:04 GMT
< Server: gunicorn/19.9.0
< 
< {
<   "uuid": "ba0a69b3-25d0-415e-b998-030120052f4d"
< }
< 

* 

```

* use `requests.New()` method to create a global session for http client.
* use `requests.URL()` method to define a sever address to request.
* use `requests.Trace()` method to open http trace mode.
* use `DoRequest()` method to send a request from local to remote server.

Also, you can using simple method like `requests.Get()`, `requests.Post()` etc. to send a request,
and return `*http.Response`, `error`. This is fully compatible with `net/http` method.

#### Simple Get
```go
package main

import (
	"bytes"
	"github.com/golang-io/requests"
	"log"
)

func main() {
	resp, err := requests.Get("https://httpbin.org/get")
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		log.Fatalln(err)
	}
	log.Println(buf.String())
}
```

```shell
% go run github.com/golang-io/examples/example_2/main.go
2024/03/22 20:31:12 {
  "args": {}, 
  "headers": {
    "Host": "httpbin.org", 
    "User-Agent": "Go-http-client/1.1", 
    "X-Amzn-Trace-Id": "Root=1-65fd7a10-781981cc111ac4662510a87e"
  }, 
  "origin": "43.132.141.21", 
  "url": "https://httpbin.org/get"
}

```

#### Auto handle `response.Body`

There are many negative cases, network connections not released or memory cannot be released,
because the `response.Body` is not closed correctly. To solve this problem, `requests` offers
type `*requests.Response`. The response body sample read from `response.Content`.
There is no need to declare a bunch of variables and duplicate code just for reading the `response.Body`.
Additionally, the body will be safely closed, regardless of whether you need to read it or not.

For example:

```go
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

```
```shell
% go run github.com/golang-io/examples/example_3/main.go
2024/03/22 20:43:25 resp={
  "args": {}, 
  "data": "Hello world", 
  "files": {}, 
  "form": {}, 
  "headers": {
    "Content-Length": "11", 
    "Host": "httpbin.org", 
    "User-Agent": "Go-http-client/1.1", 
    "X-Amzn-Trace-Id": "Root=1-65fd7ced-718974b7528527911b977e1e"
  }, 
  "json": null, 
  "origin": "127.0.0.1", 
  "url": "http://httpbin.org/post"
}
, err=<nil>

```

### Usage

#### Common Rules
All parameters can be set at two levels: session and request. 

**Session parameters** are persistent and apply to all requests made with that session:
```go
// Create a session with persistent parameters
sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.Header("User-Agent", "My Custom Agent"),
    requests.Timeout(10),
)

// All requests from this session will inherit these parameters
resp1, _ := sess.DoRequest(context.TODO(), requests.Path("/users"))
resp2, _ := sess.DoRequest(context.TODO(), requests.Path("/posts"))
```

**Request parameters** are provisional and only apply to the specific request:
```go
sess := requests.New(requests.URL("https://api.example.com"))

// This parameter only applies to this specific request
resp, _ := sess.DoRequest(context.TODO(), 
    requests.Path("/users"),
    requests.Query("limit", "10"),
)
```

#### Debugging - Log/Trace

Requests provides multiple trace levels to help with debugging:

```go
package main

import (
	"context"
	"github.com/golang-io/requests"
)

func main() {
	sess := requests.New(
		requests.URL("https://httpbin.org/get"),
		requests.Trace(),  // Most detailed tracing
	)
	
	// The trace output will show detailed connection info
	resp, _ := sess.DoRequest(context.TODO())
}
```

You can also use custom loggers:

```go
sess := requests.New(
    requests.URL("https://httpbin.org/get"),
    requests.Logger(myCustomLogger),
)
```

#### Set Body

Requests supports multiple body formats:

```go
// String body
resp, _ := sess.DoRequest(context.TODO(), 
    requests.MethodPost,
    requests.Body("Hello World"),
)

// JSON body (automatically serialized)
resp, _ := sess.DoRequest(context.TODO(), 
    requests.MethodPost,
    requests.JSON(map[string]interface{}{
        "name": "John",
        "age": 30,
    }),
)

// Form data
resp, _ := sess.DoRequest(context.TODO(), 
    requests.MethodPost,
    requests.Form(map[string]string{
        "username": "johndoe",
        "password": "secret",
    }),
)

// File upload
resp, _ := sess.DoRequest(context.TODO(), 
    requests.MethodPost,
    requests.FileUpload("file", "/path/to/file.pdf", "application/pdf"),
)
```

#### Set Header

Headers can be set at both session and request levels:

```go
// Session-level headers
sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.Header("Authorization", "Bearer token123"),
    requests.Header("Accept", "application/json"),
)

// Request-level headers (will override session headers if same key)
resp, _ := sess.DoRequest(context.TODO(),
    requests.Header("X-Custom-Header", "value"),
    requests.Headers(map[string]string{
        "Content-Type": "application/json",
        "X-Request-ID": "abc123",
    }),
)
```

#### Authentication

Requests supports various authentication methods:

```go
// Basic authentication
sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.BasicAuth("username", "password"),
)

// Bearer token
sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.BearerAuth("your-token-here"),
)

// Custom auth scheme
sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.Auth("CustomScheme", "credentials"),
)
```

#### Gzip Compression

Requests handles gzip compression automatically:

```go
// Automatic handling of gzip responses
sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.AutoDecompress(true),  // Default is true
)

// Send compressed request
resp, _ := sess.DoRequest(context.TODO(),
    requests.MethodPost,
    requests.Gzip("large content here"),
)
```

#### Request and Response Middleware

Add middleware to process requests or responses:

```go
// Request middleware
sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.RequestMiddleware(func(req *http.Request) error {
        req.Header.Add("X-Request-Time", time.Now().String())
        return nil
    }),
)

// Response middleware
sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.ResponseMiddleware(func(resp *http.Response) error {
        log.Printf("Response received with status: %d", resp.StatusCode)
        return nil
    }),
)
```

#### Client and Transport Middleware

Customize the underlying HTTP client behavior:

```go
// Custom transport
transport := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 10,
    IdleConnTimeout:     30 * time.Second,
}

sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.Transport(transport),
)

// Transport middleware
sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.TransportMiddleware(func(rt http.RoundTripper) http.RoundTripper {
        return requests.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
            startTime := time.Now()
            resp, err := rt.RoundTrip(req)
            duration := time.Since(startTime)
            log.Printf("Request took %v", duration)
            return resp, err
        })
    }),
)
```

#### Proxy

Configure proxies for your requests:

```go
// HTTP proxy
sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.Proxy("http://proxy.example.com:8080"),
)

// SOCKS5 proxy
sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.Proxy("socks5://proxy.example.com:1080"),
)

// Per-request proxy
resp, _ := sess.DoRequest(context.TODO(),
    requests.Proxy("http://special-proxy.example.com:8080"),
)
```

#### Retry

Automatic retry functionality for failed requests:

```go
sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.Retry(3, 500*time.Millisecond),  // 3 retries with 500ms delay
    // Custom retry condition
    requests.RetryCondition(func(resp *http.Response, err error) bool {
        return err != nil || resp.StatusCode >= 500
    }),
)
```

#### Sessions with Cookies

Maintain a persistent session with cookies:

```go
// Create a session that automatically handles cookies
sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.CookieJar(true),  // Enable cookie jar
)

// Login to establish session
_, _ = sess.DoRequest(context.TODO(),
    requests.MethodPost,
    requests.Path("/login"),
    requests.Form(map[string]string{
        "username": "johndoe",
        "password": "secret",
    }),
)

// Subsequent requests will include session cookies automatically
resp, _ := sess.DoRequest(context.TODO(),
    requests.Path("/profile"),
)
```

#### Streaming Downloads

Handle large file downloads efficiently:

```go
package main

import (
	"context"
	"github.com/golang-io/requests"
	"io"
	"os"
)

func main() {
	file, _ := os.Create("large-file.zip")
	defer file.Close()
	
	sess := requests.New(requests.URL("https://example.com/large-file.zip"))
	
	// Stream the download directly to file
	resp, _ := sess.DoRequest(context.TODO(), requests.Stream(true))
	
	_, _ = io.Copy(file, resp.Body)
	resp.Body.Close()
}
```

## Advanced Examples

### REST API Client

```go
package main

import (
	"context"
	"fmt"
	"github.com/golang-io/requests"
	"log"
)

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func main() {
	apiClient := requests.New(
		requests.URL("https://jsonplaceholder.typicode.com"),
		requests.Header("Accept", "application/json"),
		requests.Timeout(10),
	)
	
	// GET users
	resp, err := apiClient.DoRequest(context.TODO(),
		requests.Path("/users"),
	)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	
	var users []User
	if err := resp.JSON(&users); err != nil {
		log.Fatalf("Parse error: %v", err)
	}
	
	fmt.Printf("Found %d users\n", len(users))
	
	// POST a new user
	newUser := User{Name: "John Doe", Email: "john@example.com"}
	resp, err = apiClient.DoRequest(context.TODO(),
		requests.MethodPost,
		requests.Path("/users"),
		requests.JSON(newUser),
	)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	
	var createdUser User
	if err := resp.JSON(&createdUser); err != nil {
		log.Fatalf("Parse error: %v", err)
	}
	
	fmt.Printf("Created user with ID: %d\n", createdUser.ID)
}
```

## Server

```go
package main

import (
	"context"
	"fmt"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-io/requests"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net/http"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func ws(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Failed to read message: %v", err)
			break
		}

		log.Printf("Received message: %s", message)

		err = conn.WriteMessage(messageType, message)
		if err != nil {
			log.Printf("Failed to write message: %v", err)
			break
		}
	}
}

func main() {
	r := requests.NewServeMux(
		requests.URL("0.0.0.0:1234"),
		requests.Use(middleware.Recoverer, middleware.Logger),
		requests.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r)
			})
		}),
	)
	r.Route("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("panic test")
	})
	r.Route("/echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, r.Body)
	})
	r.Route("/ping", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "pong\n")
	}, requests.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}))
	r.Route("/ws", ws)
	err := requests.ListenAndServe(context.Background(), r)
	fmt.Println(err)
}
```
Then, do some requests...
```shell
% curl http://127.0.0.1:1234/echo
% curl http://127.0.0.1:1234/ping
pong

```
And, there are some logs from server.
```shell
% go run github.com/golang-io/examples/server/example_1/main.go
2024-03-27 02:47:47 http serve 0.0.0.0:1234
2024/03/27 02:47:59 "GET http://127.0.0.1:1234/echo HTTP/1.1" from 127.0.0.1:60922 - 000 0B in 9.208µs
path use {}
2024/03/27 02:48:06 "GET http://127.0.0.1:1234/ping HTTP/1.1" from 127.0.0.1:60927 - 200 5B in 5.125µs

```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.

## Acknowledgments
