// Package requests 提供了唯一ID生成功能
// Package requests provides unique ID generation functionality
package requests

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"time"
)

// GenId 生成唯一的随机ID（使用Base36编码）
// 如果提供了id参数且非空，则直接返回该id
// 否则基于当前纳秒时间戳和随机数生成唯一ID
//
// GenId generates a unique random ID (using Base36 encoding)
// If an id parameter is provided and not empty, returns that id directly
// Otherwise generates a unique ID based on current nanosecond timestamp and random number
//
// 参数 / Parameters:
//   - id: ...string - 可选的自定义ID（如果提供且非空，则直接使用） / Optional custom ID (if provided and not empty, use directly)
//
// 返回值 / Returns:
//   - string: 生成的唯一ID（大写Base36格式） / Generated unique ID (uppercase Base36 format)
//
// ID生成算法 / ID Generation Algorithm:
//   - 使用纳秒级时间戳 * 1000 + 随机数(0-999) / Uses nanosecond timestamp * 1000 + random(0-999)
//   - 转换为Base36编码（0-9, A-Z） / Converts to Base36 encoding (0-9, A-Z)
//   - 结果转换为大写字母 / Result converted to uppercase
//
// 使用场景 / Use Cases:
//   - 生成请求唯一标识符（Request-Id） / Generate unique request identifiers (Request-Id)
//   - 分布式系统追踪 / Distributed system tracing
//   - 日志关联 / Log correlation
//   - 事务ID生成 / Transaction ID generation
//
// 示例 / Example:
//
//	// 生成随机ID
//	// Generate random ID
//	id := requests.GenId()
//	fmt.Printf("Generated ID: %s\n", id) // 例如：2F8K3L9M4N7P
//
//	// 使用自定义ID
//	// Use custom ID
//	customId := requests.GenId("my-custom-id")
//	fmt.Printf("Custom ID: %s\n", customId) // 输出：my-custom-id
//
//	// 在请求中使用
//	// Use in requests
//	resp, _ := session.DoRequest(ctx,
//	    requests.URL("http://example.com"),
//	    requests.Header("Request-Id", requests.GenId()),
//	)
func GenId(id ...string) string {
	if len(id) != 0 && id[0] != "" {
		return id[0]
	}
	// 受 strconv.ParseUint(id, 36, 64) 及 ID 长度约束，ID 必须落在 uint64 范围内。
	// 若用纳秒精度，时间戳已占约 61 bit，仅剩约 3 bit 给随机，并发场景下熵不足、
	// 易发生碰撞。这里改用微秒精度，并通过乘法做"高位时间 + 低位随机"的干净分隔：
	//   - UnixMicro() * 1000 将微秒时间戳抬高 3 个十进制位作为高位，保证时间单调性；
	//   - 低 3 位（[0,1000) 随机数）专门用于区分同一微秒内的并发调用，不会覆盖时间位。
	// 微秒时间戳约 51 bit，*1000 后约 61 bit，远在 uint64 范围内，不会溢出。
	//
	// Constrained by strconv.ParseUint(id, 36, 64) and ID length, the value must fit in
	// uint64. With nanosecond precision the timestamp alone uses ~61 bits, leaving only
	// ~3 bits for randomness, which is too little under concurrency. We use microsecond
	// precision and a multiply to cleanly separate the high time bits from the low
	// random bits (so randomness never overwrites the time bits): UnixMicro()*1000 keeps
	// IDs monotonic, and the low [0,1000) random part disambiguates concurrent calls
	// within the same microsecond.
	i := uint64(time.Now().UnixNano())*1000 + uint64(rand.Int64N(1000))
	return strings.ToUpper(strconv.FormatUint(i, 36))
}
