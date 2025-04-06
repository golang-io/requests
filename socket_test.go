package requests

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func TestSocket(t *testing.T) {
	// 启动测试服务器
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("创建监听器失败: %v", err)
	}
	defer listener.Close()
	// 在后台接受连接
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// 简单的回显服务
		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		conn.Write(buf[:n])
	}()

	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
	}{
		{
			name: "TCP正常连接",
			opts: []Option{
				URL("tcp://" + listener.Addr().String()),
				Timeout(time.Second),
			},
			wantErr: false,
		},
		{
			name: "无效URL",
			opts: []Option{
				URL("invalid://localhost"),
				Timeout(time.Second),
			},
			wantErr: true,
		},
		{
			name: "错误URL",
			opts: []Option{
				URL("://:::"),
				Timeout(time.Second),
			},
			wantErr: true,
		},
		{
			name: "连接超时",
			opts: []Option{
				URL("tcp://240.0.0.1:12345"), // 不可达的地址
				Timeout(1),
			},
			wantErr: true,
		},
		{
			name: "Unix socket连接",
			opts: []Option{
				URL("unix:///tmp/test.sock"),
				Timeout(time.Second),
			},
			wantErr: true, // Unix socket文件不存在，应该失败
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			conn, err := Socket(ctx, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Socket() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				defer conn.Close()
				// 测试连接是否可用
				_, err = conn.Write([]byte("test"))
				if err != nil {
					t.Errorf("写入数据失败: %v", err)
				}
				buf := make([]byte, 4)
				_, err = conn.Read(buf)
				if err != nil {
					t.Errorf("读取数据失败: %v", err)
				}
				if string(buf) != "test" {
					t.Errorf("期望读取到 'test'，得到 %s", string(buf))
				}
			}
		})
	}
}

func TestSocket_ContextCancel(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("创建监听器失败: %v", err)
	}
	defer listener.Close()
	// 在后台接受连接
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// 简单的回显服务
		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		conn.Write(buf[:n])
	}()
	// 创建一个可取消的上下文
	ctx, cancel := context.WithCancel(context.Background())

	// 立即取消
	cancel()

	// 尝试建立连接
	if _, err = Socket(ctx, URL("tcp://"+listener.Addr().String())); errors.Is(err, context.Canceled) {
		t.Log(err)
		return
	}
	t.Errorf("期望错误为 context.Canceled，得到 %v", err)

}

func TestSocket_WithCustomDialer(t *testing.T) {
	// 测试自定义本地地址
	localAddr := &net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 0, // 系统自动分配端口
	}

	// 启动测试服务器
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("创建监听器失败: %v", err)
	}
	defer listener.Close()

	// 使用自定义本地地址建立连接
	conn, err := Socket(context.Background(),
		URL("tcp://"+listener.Addr().String()),
		LocalAddr(localAddr),
	)

	if err != nil {
		t.Fatalf("建立连接失败: %v", err)
	}
	defer conn.Close()

	// 验证连接的本地地址
	localAddrStr := conn.LocalAddr().String()
	if !strings.Contains(localAddrStr, "127.0.0.1") {
		t.Errorf("期望本地地址为 127.0.0.1，得到 %s", localAddrStr)
	}
}
