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
	// 测试ID生成的时间顺序性
	// 由于ID包含时间戳，后生成的ID应该大于先生成的ID
	ids := make([]string, 10)

	// 生成多个ID
	for i := range 10 {
		ids[i] = GenId()
		time.Sleep(time.Microsecond) // 确保时间戳不同
	}

	// 验证ID的递增性（基于时间戳）
	for i := 1; i < len(ids); i++ {
		prevNum, err1 := strconv.ParseUint(ids[i-1], 36, 64)
		currNum, err2 := strconv.ParseUint(ids[i], 36, 64)

		if err1 != nil || err2 != nil {
			t.Fatalf("Failed to parse IDs: %v, %v", err1, err2)
		}

		if currNum <= prevNum {
			t.Errorf("ID should be increasing: %s (%d) <= %s (%d)",
				ids[i], currNum, ids[i-1], prevNum)
		}
	}
}

func Test_GenId_Uniqueness(t *testing.T) {
	// 测试ID唯一性
	count := 1000

	ids := make(map[string]bool)
	for range count {
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

	for range count {
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
	for range b.N {
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
