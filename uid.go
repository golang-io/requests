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
	i := time.Now().UnixNano()*1000 + rand.Int64N(1000) // % 4738381338321616895
	return strings.ToUpper(strconv.FormatUint(uint64(i), 36))
}
