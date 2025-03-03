package requests

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"time"
)

// GenId gen random id.
func GenId(id ...string) string {
	if len(id) != 0 && id[0] != "" {
		return id[0]
	}
	i := time.Now().UnixMicro()*1000 + rand.Int64N(1000) // % 4738381338321616895
	return strings.ToUpper(strconv.FormatUint(uint64(i), 36))
}
