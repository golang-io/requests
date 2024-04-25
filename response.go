package requests

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"time"
)

// Response wrap std response
type Response struct {
	*http.Request
	*http.Response
	StartAt time.Time
	Cost    time.Duration
	Content *bytes.Buffer
	Err     error
}

func newResponse() *Response {
	return &Response{StartAt: time.Now(), Response: &http.Response{}, Content: &bytes.Buffer{}}
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
func streamRead(reader io.Reader, f func(int64, []byte) error) (int64, error) {
	i, cnt, r := int64(0), int64(0), bufio.NewReaderSize(reader, 1024*1024)
	for {
		raw, err1 := r.ReadBytes(10) // ascii('\n') = 10
		if err1 != nil && err1 != io.EOF {
			return cnt, err1
		}
		// 保证最后一行能被处理，并且可以正常返回
		i, cnt = i+1, cnt+int64(len(raw))
		if err2 := f(i, raw); err1 == io.EOF || err2 != nil {
			return cnt, err2
		}
	}
}
