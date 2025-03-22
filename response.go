package requests

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"time"
)

// Response 包装了 http.Response 结构体，提供了额外的功能：
// 1. 记录请求开始时间和耗时
// 2. 缓存响应内容
// 3. 错误处理
// 4. 统计信息收集
type Response struct {
	*http.Request                // 原始 HTTP 请求
	*http.Response               // 原始 HTTP 响应
	StartAt        time.Time     // 请求开始时间
	Cost           time.Duration // 请求耗时
	Content        *bytes.Buffer // 响应内容缓存
	Err            error         // 请求过程中的错误
}

// newResponse 创建一个新的 Response 实例
// 参数 r 是原始的 HTTP 请求
func newResponse(r *http.Request) *Response {
	return &Response{
		Request:  r,
		StartAt:  time.Now(),
		Response: &http.Response{},
		Content:  &bytes.Buffer{},
	}
}

// String 实现 fmt.Stringer 接口
// 返回响应内容的字符串形式
func (resp *Response) String() string {
	return resp.Content.String()
}

// Error 实现 error 接口
// 返回请求过程中的错误信息
func (resp *Response) Error() string {
	if resp.Err == nil {
		return ""
	}
	return resp.Err.Error()
}

// Stat 返回请求的统计信息
// 包括请求/响应的详细信息、耗时等
func (resp *Response) Stat() *Stat {
	return responseLoad(resp)
}

// streamRead 按行读取数据流
// reader: 输入的数据流
// fn: 处理每一行数据的回调函数，参数为行号和行内容
// 返回值：读取的总字节数和可能的错误
func streamRead(reader io.Reader, fn func(int64, []byte) error) (int64, error) {
	// 创建一个 1MB 缓冲的读取器
	i, cnt, r := int64(0), int64(0), bufio.NewReaderSize(reader, 1024*1024)
	for {
		// 读取直到遇到换行符
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
