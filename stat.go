package requests

import (
	"encoding/json"
	"fmt"
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
