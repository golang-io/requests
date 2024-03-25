package requests

import (
	"encoding/json"
	"fmt"
)

const RequestId = "Request-Id"

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
	Err string `json:"Err"`
}

// String implement fmt.Stringer interface.
func (stat *Stat) String() string {
	b, _ := json.Marshal(stat)
	return string(b)
}

const dateTime = "2006-01-02 15:04:05.000"

// StatLoad stat.
func StatLoad(resp *Response) *Stat {
	stat := &Stat{
		StartAt: resp.StartAt.Format(dateTime),
		Cost:    resp.Cost.Milliseconds(),
	}
	if resp.Response != nil {
		var err error
		if resp.Content == nil || resp.Content.Len() == 0 {
			if resp.Content, err = CopyResponseBody(resp.Response); err != nil {
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

			buf, _, err := copyBody(body)
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
