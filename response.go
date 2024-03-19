package requests

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"os"
	"time"
)

// Response wrap std response
type Response struct {
	*http.Response
	*http.Request
	StartAt time.Time
	Cost    time.Duration
	Content *bytes.Buffer
	Err     error
}

func newResponse() *Response {
	return &Response{StartAt: time.Now(), Content: &bytes.Buffer{}}
}

func (resp *Response) String() string {
	return resp.Content.String()
}

func (resp *Response) Error() string {
	if resp.Err == nil {
		return ""
	}
	return resp.Err.Error()
}

// Text parse to string
func (resp *Response) Text() string {
	return resp.Content.String()
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
	return f.Write(resp.Content.Bytes())
}

func (resp *Response) Stat() *Stat {
	return StatLoad(resp)
}

// streamRead
func streamRead(reader io.Reader, f func(int64, []byte) error) (int64, error) {
	i, cnt, r := int64(0), int64(0), bufio.NewReaderSize(reader, 1024*1024)
	for {
		raw, err := r.ReadBytes(10) // ascii('\n') = 10
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
