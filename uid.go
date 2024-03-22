package requests

import (
	"math/rand"
	"strconv"
	"strings"
	"time"
)

var source = rand.New(rand.NewSource(time.Now().UnixNano()))

// GenId gen random id.
func GenId(id ...string) string {
	if len(id) != 0 && id[0] != "" {
		return id[0]
	}
	s := uint64(time.Now().UnixMicro()*1000 + source.Int63n(1000)) // % 4738381338321616895
	return strings.ToUpper(strconv.FormatUint(s, 36))
}
