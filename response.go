package requests

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"time"
)

// Response wrap `http.Response` struct.
type Response struct {
	*http.Request
	*http.Response
	StartAt time.Time
	Cost    time.Duration
	Content *bytes.Buffer
	Err     error
}

func newResponse(r *http.Request) *Response {
	return &Response{Request: r, StartAt: time.Now(), Response: &http.Response{}, Content: &bytes.Buffer{}}
}

// String implement fmt.Stringer interface.
func (resp *Response) String() string {
	return resp.Content.String()
}

// Error implement error interface.
func (resp *Response) Error() string {
	if resp.Err == nil {
		return ""
	}
	return resp.Err.Error()
}

// Stat stat
func (resp *Response) Stat() *Stat {
	return responseLoad(resp)
}

// streamRead xx
func streamRead(reader io.Reader, fn func(int64, []byte) error) (int64, error) {
	i, cnt, r := int64(0), int64(0), bufio.NewReaderSize(reader, 1024*1024)
	for {
		raw, err1 := r.ReadBytes(10) // ascii('\n') = 10
		if err1 != nil && err1 != io.EOF {
			return cnt, err1
		}
		// 保证最后一行能被处理，并且可以正常返回
		i, cnt = i+1, cnt+int64(len(raw))
		if err2 := fn(i, raw); err1 == io.EOF || err2 != nil {
			return cnt, err2

		}
	}
}

// streamRoundTrip 创建一个流式处理的RoundTripper中间件
// 参数 f 是流处理回调函数，接收两个参数：
//   - i int64: 当前处理的数据块序号（从1开始）
//   - raw []byte: 原始数据块内容（按换行符分割）
//
// 返回值是可用于HTTP客户端中间件链的RoundTripper
// 适用场景：大文件下载、实时事件流处理等需要边接收边处理的场景
// 注意：与普通RoundTripper不同，此方法会流式处理响应体而不是缓存全部内容
func streamRoundTrip(fn func(i int64, raw []byte) error) func(http.RoundTripper) http.RoundTripper {
	return func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			resp := newResponse(r)
			if resp.Response, resp.Err = next.RoundTrip(r); resp.Err != nil {
				return resp.Response, resp.Err
			}

			if resp.Response.ContentLength, resp.Err = streamRead(resp.Response.Body, fn); resp.Err != nil {
				return resp.Response, resp.Err
			}
			resp.Response.Body = io.NopCloser(bytes.NewReader([]byte("[stream]")))
			return resp.Response, resp.Err
		})
	}
}
