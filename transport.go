package requests

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
	"time"
)

type Transport struct {
	client  *http.Client
	options Options
}

func (tr *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := tr.request(req)
	return resp.Response, err
}

func (tr *Transport) request(req *http.Request) (*Response, error) {
	resp := &Response{StartAt: time.Now(), Request: req, Response: &http.Response{}}
	req.Header.Add(RequestId, GenId())

	defer func(resp *Response) {
		resp.Cost = time.Since(resp.StartAt)
		if tr.options.Logf != nil {
			tr.options.Logf(context.TODO(), resp.Stat()) // 这里context,需要分开以免request cancel了，导致日志也cancel了
		}
	}(resp)

	if tr.options.TraceLv > 0 {
		resp.Response, resp.Err = tr.Trace(req)
	} else {
		// 这里用RoundTrip需要处理timeout的问题和代理无效的问题
		//resp.Response, resp.Err = tr.client.Transport.RoundTrip(req)
		resp.Response, resp.Err = tr.client.Do(req)
	}

	if resp.Err != nil {
		if errors.Is(resp.Err, context.DeadlineExceeded) {
			resp.Err = fmt.Errorf("doRequest: err=%w, timeout=%s", resp.Err, tr.options.Timeout)
			return resp, resp.Err
		}
		return resp, fmt.Errorf("doRequest: %w, %#v", resp.Err, resp.Err)
	}

	if resp.Response == nil || resp.Response.Body == nil {
		resp.Err = fmt.Errorf("resp.Body is nil")
		return resp, resp.Err
	}

	defer resp.Response.Body.Close()

	for _, each := range tr.options.ResponseEach {
		if err := each(resp.Context(), resp.Response); err != nil {
			return resp, fmt.Errorf("responseEach: %w", err)
		}
	}

	if tr.options.Stream != nil {
		resp.Response.ContentLength, resp.Err = resp.stream(tr.options.Stream)
	} else {
		resp.Response.ContentLength, resp.Err = resp.body.ReadFrom(resp.Response.Body)
		resp.Response.Body = io.NopCloser(bytes.NewReader(resp.body.Bytes()))
	}
	return resp, resp.Err
}

// RoundTrip trace a request
func (tr *Transport) Trace(req *http.Request) (*http.Response, error) {
	ctx := httptrace.WithClientTrace(req.Context(), trace)
	req.Header.Set(RequestId, GenId())

	req2 := req.WithContext(ctx)
	reqLog, err := DumpRequest(req2)
	if err != nil {
		Log("request error: %w", err)
		return nil, err
	}
	resp, err := tr.client.Transport.RoundTrip(req2)

	if tr.options.TraceLv >= 2 {
		Log(show(reqLog, "> ", tr.options.TraceLimit))
	}
	if err != nil {
		return nil, err
	}

	respLog, err := httputil.DumpResponse(resp, true)
	if err != nil {
		return nil, err
	}
	if tr.options.TraceLv >= 3 {
		Log(show(respLog, "< ", tr.options.TraceLimit))
	}
	return resp, nil
}

const RequestId = "Request-Id"
