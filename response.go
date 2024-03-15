package requests

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"time"
)

// Stat stats
type Stat struct {
	RequestId string
	StartAt   string `json:"StartAt"`
	Cost      int64  `json:"Cost"`
	Request   struct {
		Method string            `json:"Method"`
		Header map[string]string `json:"Header"`
		URL    string            `json:"URL"`
		Body   any               `json:"Body"`
	} `json:"Request"`
	Response struct {
		Header        map[string]string `json:"Header"`
		Body          any               `json:"Body"`
		StatusCode    int               `json:"StatusCode"`
		ContentLength int64             `json:"ContentLength"`
	} `json:"Response"`
	Err   string `json:"Err"`
	Retry int    `json:"Retry"`
}

func (stat Stat) String() string {
	b, _ := json.Marshal(stat)
	return string(b)
}

// Response wrap std response
type Response struct {
	*http.Response
	*http.Request // 这里是为了，保证存在请求发起失败的情况下，response=nil，request还能获取到原始记录
	StartAt       time.Time
	Cost          time.Duration
	body          bytes.Buffer
	Retry         int
	Err           error
}

const dateTime = "2006-01-02 15:04:05.000"

// Stat stat
func (resp *Response) Stat() Stat {

	stat := Stat{
		StartAt: resp.StartAt.Format(dateTime),
		Cost:    resp.Cost.Milliseconds(),
	}

	if resp.Response != nil {
		body := make(map[string]any)

		if err := json.Unmarshal(resp.body.Bytes(), &body); err != nil {
			stat.Response.Body = resp.body.String()
		} else {
			stat.Response.Body = body
		}

		stat.Response.Header = make(map[string]string)

		for k, v := range resp.Response.Header {
			stat.Response.Header[k] = v[0]
		}
		stat.Response.ContentLength = resp.Response.ContentLength
		stat.Response.StatusCode = resp.StatusCode

	}

	if resp.Request != nil {
		stat.RequestId = resp.Request.Header.Get(RequestId)
		stat.Request.Method = resp.Request.Method
		stat.Request.URL = resp.Request.URL.String()
		if resp.Request.GetBody != nil {
			body, _ := resp.Request.GetBody()

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(body)

			m := make(map[string]any)

			if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
				stat.Request.Body = buf.String()
			} else {
				stat.Request.Body = m
			}
		}

		stat.Request.Header = make(map[string]string)

		for k, v := range resp.Request.Header {
			stat.Request.Header[k] = v[0]
		}
	}

	if resp.Err != nil {
		stat.Err = resp.Err.Error()
	}

	return stat
}

func (resp *Response) String() string {
	return resp.Text()
}

func (resp *Response) Error() string {
	if resp.Err == nil {
		return ""
	}
	return resp.Err.Error()
}

// StdLib return net/http.Response
func (resp *Response) StdLib() *http.Response {
	return resp.Response
}

// Text parse to string
func (resp *Response) Text() string {
	return resp.body.String()
}

// Bytes to bytes
func (resp *Response) Bytes() []byte {
	return resp.body.Bytes()
}

// Download parse response to a file
func (resp *Response) Download(name string) (int, error) {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return 0, err
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)
	return f.Write(resp.body.Bytes())
}

// JSON parse response.
// Deprecated: DO NOT USE IT. Because it's not compatible with the standard library.
func (resp *Response) JSON(v any) error {
	return json.Unmarshal(resp.body.Bytes(), v)
}

// Dump returns the given request in its HTTP/1.x wire representation.
func (resp *Response) Dump() ([]byte, error) {
	return httputil.DumpResponse(resp.Response, true)
}

func (resp *Response) stream(f func(int64, []byte) error) (int64, error) {
	i, cnt, reader := int64(0), int64(0), bufio.NewReaderSize(resp.Response.Body, 1024*1024)
	for {
		raw, err := reader.ReadBytes(10) // ascii('\n') = 10
		if err != nil {
			if err != io.EOF {
				return cnt, err
			}
			return cnt, nil
		}
		i, cnt = i+1, cnt+int64(len(raw))
		if err = f(i, raw); err != nil {
			return cnt, err
		}
	}
}
