package requests

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTrace(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message":"test response"}`))
	}))
	defer server.Close()

	// 创建一个新的会话并启用跟踪
	sess := New(
		URL(server.URL),
		Trace(100), // 设置较小的限制以测试截断功能
	)

	// 发送请求
	resp, err := sess.DoRequest(context.Background())
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	// 验证响应
	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", resp.StatusCode)
	}
}

func TestShow(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		input   []byte
		limit   int
		want    string
		wantLen int
	}{
		{
			name:    "正常输出不截断",
			prompt:  "> ",
			input:   []byte("test\ndata"),
			limit:   100,
			want:    "> test\n> data\n",
			wantLen: 14,
		},
		{
			name:    "超出限制截断",
			prompt:  "* ",
			input:   []byte(strings.Repeat("a", 200)),
			limit:   50,
			want:    "* " + strings.Repeat("a", 48) + "...[Len=203, Truncated[50]]",
			wantLen: 77,
		},
		{
			name:    "处理百分号",
			prompt:  "> ",
			input:   []byte("50%"),
			limit:   100,
			want:    "> 50%\n",
			wantLen: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := show(tt.prompt, tt.input, tt.limit)
			if got != tt.want {
				t.Errorf("show() = %v, want %v", got, tt.want)
			}
			if len(got) != len(tt.want) {
				t.Errorf("len(show()) = %v, want %v", len(got), tt.wantLen)
			}
		})
	}
}

func TestTraceLv(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Hello, World!"))
	}))
	defer server.Close()

	// 创建请求
	req, _ := http.NewRequest("GET", server.URL, nil)

	tests := []struct {
		name    string
		used    bool
		limit   int
		wantErr bool
	}{
		{
			name:    "启用跟踪",
			used:    true,
			limit:   1024,
			wantErr: false,
		},
		{
			name:    "禁用跟踪",
			used:    false,
			limit:   1024,
			wantErr: false,
		},
		{
			name:    "小限制值",
			used:    true,
			limit:   10,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建跟踪包装器
			wrapper := traceLv(tt.used, tt.limit)
			transport := wrapper(http.DefaultTransport)

			// 发送请求
			resp, err := transport.RoundTrip(req)
			if (err != nil) != tt.wantErr {
				t.Errorf("RoundTrip() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				defer resp.Body.Close()
				_, _ = io.ReadAll(resp.Body)
			}
		})
	}
}

func TestLog(t *testing.T) {
	// 保存原始的标准输出
	oldStdout := stdout
	defer func() { stdout = oldStdout }()

	// 创建一个 buffer 来捕获输出
	var buf bytes.Buffer
	stdout = &buf

	// 测试不同类型的日志输出
	tests := []struct {
		name   string
		format string
		args   []interface{}
		want   string
	}{
		{
			name:   "简单字符串",
			format: "test message",
			args:   nil,
			want:   "test message\n",
		},
		{
			name:   "带参数",
			format: "value: %d",
			args:   []interface{}{42},
			want:   "value: 42\n",
		},
		{
			name:   "多个参数",
			format: "%s: %d, %v",
			args:   []interface{}{"test", 123, true},
			want:   "test: 123, true\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			Log(tt.format, tt.args...)
			if got := buf.String(); got != tt.want {
				t.Errorf("Log() = %v, want %v", got, tt.want)
			}
		})
	}
}

// 为了测试 Log 函数，我们需要一个可以捕获输出的变量
var stdout io.Writer = nil

func init() {
	stdout = io.Discard // 默认丢弃输出
}

// 重写 print 函数以使用我们的 stdout 变量
func print(s string) {
	if stdout != nil {
		stdout.Write([]byte(s))
	}
}
