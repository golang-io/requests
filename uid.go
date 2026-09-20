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
	// 当前实现：UnixNano()*1000 + [0,1000) 随机数，再 FormatUint(..., 36)。
	// 说明与局限：
	//   - 目标是落在 uint64 内，便于 strconv.ParseUint(id, 36, 64)；
	//   - 纳秒时间戳约 61 bit，再 *1000 会溢出 uint64，高位时间信息被截断；
	//   - 低位随机是「抽槽」而非原子占槽，同刻并发下生日悖论易碰撞，不保证唯一。
	//
	// Current implementation: UnixNano()*1000 + random in [0,1000), then FormatUint base 36.
	// Notes / limits:
	//   - Intended to stay in uint64 for strconv.ParseUint(id, 36, 64);
	//   - ~61-bit nanosecond timestamp *1000 overflows uint64 and truncates high time bits;
	//   - Random picks a slot; it does not reserve one, so concurrent calls can collide
	//     (birthday paradox). Uniqueness is not guaranteed.
	i := uint64(time.Now().UnixNano())*1000 + uint64(rand.Int64N(1000))
	return strings.ToUpper(strconv.FormatUint(i, 36))
}
