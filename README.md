# request

**Requests** is a simle, yet elegant, Go HTTP client library for Humans™ ✨🍰✨

#### API Reference and User Guide available on [Read the Docs](https://pkg.go.dev/github.com/golang-io/requests)
#### Supported Features & Best–Practices
* Safe Close `resp.Body`
* Only depends on standard library
* Streaming Downloads
* Chunked HTTP Requests
* Keep-Alive & Connection Pooling
* Sessions with Cookie Persistence
* Basic & Digest Authentication
* Implement http.RoundTripper fully compatible `net/http`


### Quick Start
#### Get Started
```shell
cat examples/example_1/main.go
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
$ go run examples/example_1/main.go
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
* use `requests.TraceLv()` method to open http trace mode.
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
% go run examples/example_2/main.go
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
% go run examples/example_3/main.go
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
All params can set by two level, session and request. 
session params is persistence, which is set by all request from this session.
Request params is provisional, which is set by one request from this session.

Such as `session` params:
``

Such as `request` params:
``

#### Debugging - Log/Trace

#### Set Body
#### Set Header
#### Authentication
#### Gzip compress
#### Request and Response Middleware
#### Client and Transport Middleware
#### Proxy
#### Retry
#### Transport and RoundTripper

### Example