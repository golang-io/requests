package requests

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

func uFormat(s string) int64 {
	tsm, err := strconv.ParseInt(s, 36, 64)
	fmt.Println("uFormat", err)
	return tsm
}

func Test_Id(t *testing.T) {
	now := time.Now().UnixMicro()
	nowLength := len(fmt.Sprintf("%d", now))
	t.Logf("当前时间戳: now=%d, 时间戳长度: nowLength=%d", now, nowLength)
	v, err := strconv.ParseInt(strings.Repeat("1", nowLength), 10, 64)
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	maxLength := 9 * v
	t.Logf("%d, 时间戳对应时间=%s", maxLength, time.UnixMicro(maxLength))

	random := uFormat("ZZZ")
	t.Logf("random=%d", random)

	t.Logf("id=%s", GenId())

}

// go test -v -bench='Benchmark_GenId' -benchmem .
func Benchmark_GenId(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenId()
	}
}
