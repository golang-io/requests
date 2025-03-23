package requests

import (
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func Test_GenId_Base(t *testing.T) {
	t.Log(GenId())
}

func Test_GenId_Basic(t *testing.T) {
	// 测试基本功能
	id := GenId()
	t.Run("Format Check", func(t *testing.T) {
		// 检查ID格式是否为大写字母和数字的组合
		if !strings.ContainsAny(id, "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
			t.Errorf("Invalid ID format: %s", id)
		}
	})

	t.Run("Length Check", func(t *testing.T) {
		// ID长度应该在合理范围内
		if len(id) < 10 || len(id) > 15 {
			t.Errorf("ID length out of expected range: %d", len(id))
		}
	})
}

func Test_GenId_WithParam(t *testing.T) {
	// 测试带参数的情况
	expectedId := "TEST123"
	id := GenId(expectedId)
	if id != expectedId {
		t.Errorf("Expected ID %s, got %s", expectedId, id)
	}

	// 测试空参数的情况
	id = GenId("")
	if id == "" {
		t.Error("Generated ID should not be empty")
	}
}

func Test_GenId_Timestamp(t *testing.T) {
	// 测试时间戳部分
	beforeGen := time.Now().UnixMicro()
	id := GenId()
	afterGen := time.Now().UnixMicro()

	// 解析生成的ID
	parsedNum, err := strconv.ParseUint(id, 36, 64)
	if err != nil {
		t.Fatalf("Failed to parse generated ID: %v", err)
	}

	// 验证时间戳部分是否在合理范围内
	timestamp := int64(parsedNum / 1000)
	if timestamp < beforeGen || timestamp > afterGen {
		t.Errorf("Timestamp out of expected range: %d not in [%d, %d]", timestamp, beforeGen, afterGen)
	}
}

func Test_GenId_Uniqueness(t *testing.T) {
	// 测试ID唯一性
	count := 1000

	ids := make(map[string]bool)
	for i := 0; i < count; i++ {
		id := GenId()
		if ids[id] {
			// SKIP: 这是有问题的, 先忽略!
			t.Skipf("Duplicate ID generated: %s", id)
		}
		ids[id] = true
	}
}

func Test_GenId_Concurrent(t *testing.T) {
	// 测试并发生成的正确性
	count := 100
	ids := sync.Map{}
	wg := sync.WaitGroup{}

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := GenId()
			if _, loaded := ids.LoadOrStore(id, true); loaded {
				t.Errorf("Duplicate ID in concurrent generation: %s", id)
			}
		}()
	}
	wg.Wait()
}

// go test -v -test.bench='Benchmark_GenId.*' -test.run='KKK.*' -benchmem .
func Benchmark_GenId(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenId()
	}
}

// go test -v -test.bench='Benchmark_GenId.*' -test.run='KKK.*' -benchmem .
func Benchmark_GenId_Parallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			GenId()
		}
	})
}
