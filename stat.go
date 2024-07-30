package requests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const RequestId = "Request-Id"
const dateTime = "2006-01-02 15:04:05.000"

// Stat stats
type Stat struct {
	RequestId string `json:"RequestId"`
	StartAt   string `json:"StartAt"`
	Cost      int64  `json:"Cost"`

	Request struct {
		// RemoteAddr is remote addr in server side,
		// For client requests, it is unused.
		RemoteAddr string `json:"RemoteAddr"`

		// URL is Request.URL
		// For client requests, is request addr. contains schema://ip:port/path/xx
		// For server requests, is only path. eg: /api/v1/xxx
		URL    string            `json:"URL"`
		Method string            `json:"Method"`
		Header map[string]string `json:"Header"`
		Body   any               `json:"Body"`
	} `json:"Request"`
	Response struct {

		// URL is server addr(http://127.0.0.1:8080).
		// For client requests, it is unused.
		URL           string            `json:"URL"`
		Header        map[string]string `json:"Header"`
		Body          any               `json:"Body"`
		StatusCode    int               `json:"StatusCode"`
		ContentLength int64             `json:"ContentLength"`
	} `json:"Response"`
	Err string `json:"Err"`
}

// String implement fmt.Stringer interface.
func (stat *Stat) String() string {
	b, _ := json.Marshal(stat)
	return string(b)
}

// Print is used for server side
func (stat *Stat) Print() string {
	return fmt.Sprintf("%s %s \"%s -> %s%s\" - %d %dB in %dms",
		stat.StartAt, stat.Request.Method,
		stat.Request.RemoteAddr, stat.Response.URL, stat.Request.URL,
		stat.Response.StatusCode, stat.Response.ContentLength, stat.Cost)
}

// statLoad stat.
func responseLoad(resp *Response) *Stat {
	stat := &Stat{
		StartAt: resp.StartAt.Format(dateTime),
		Cost:    time.Since(resp.StartAt).Milliseconds(),
	}
	if resp.Response != nil {
		var err error
		if resp.Content == nil || resp.Content.Len() == 0 {
			if resp.Content, resp.Response.Body, err = CopyBody(resp.Response.Body); err != nil {
				stat.Err += fmt.Sprintf("read response: %s", err)
				return stat
			}
		}
		stat.Response.Body = make(map[string]any)
		if err := json.Unmarshal(resp.Content.Bytes(), &stat.Response.Body); err != nil {
			stat.Response.Body = resp.Content.String()
		}

		stat.Response.Header = make(map[string]string)
		for k, v := range resp.Response.Header {
			stat.Response.Header[k] = v[0]
		}
		stat.Response.ContentLength = resp.Response.ContentLength
		if stat.Response.ContentLength == -1 && resp.Content.Len() != 0 {
			stat.Response.ContentLength = int64(resp.Content.Len())
		}
		stat.Response.StatusCode = resp.StatusCode
	}
	if resp.Request != nil {
		stat.RequestId = resp.Request.Header.Get(RequestId)
		stat.Request.Method = resp.Request.Method
		stat.Request.URL = resp.Request.URL.String()
		if resp.Request.GetBody != nil {
			body, err := resp.Request.GetBody()
			if err != nil {
				stat.Err += fmt.Sprintf("read request1: %s", err)
				return stat
			}

			buf, err := ParseBody(body)
			if err != nil {
				stat.Err += fmt.Sprintf("read request2: %s", err)
				return stat
			}

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

func serveLoad(w *ResponseWriter, r *http.Request, start time.Time, buf *bytes.Buffer) *Stat {
	stat := &Stat{
		StartAt: start.Format("2006-01-02 15:04:05.000"),
		Cost:    time.Since(start).Milliseconds(),
	}
	stat.Request.RemoteAddr = r.RemoteAddr
	stat.Request.Method = r.Method
	stat.Request.Header = make(map[string]string)
	for k, v := range r.Header {
		stat.Request.Header[k] = v[0]
	}
	stat.Request.URL = r.URL.String()

	if buf != nil {
		m := make(map[string]any)
		if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
			stat.Request.Body = buf.String()
		} else {
			stat.Request.Body = m
		}
	}
	scheme := "http://"
	if r.TLS != nil {
		scheme = "https://"
	}
	stat.Response.URL = scheme + r.Host
	stat.Response.StatusCode = w.StatusCode
	stat.Response.ContentLength = int64(len(w.Content))
	stat.Response.Header = make(map[string]string)
	for k, v := range r.Header {
		stat.Response.Header[k] = v[0]
	}
	stat.Response.Body = string(w.Content)
	return stat
}
