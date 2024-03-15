package requests

import (
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"
)

var source = rand.New(rand.NewSource(time.Now().Unix()))
var IP = net.ParseIP("127.0.0.1")
var ipNum string

func init() {
	ifns, err := net.InterfaceAddrs()
	if err != nil {
		panic(err)
	}

	for _, addr := range ifns {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			IP = ipNet.IP.To4()
			break
		}
	}
	ipNum = strconv.FormatInt(int64(uint(IP[0])<<24|uint(IP[1])<<16|uint(IP[2])<<8|uint(IP[3]))%46655, 36)
}

// GenId gen id
// 当前时间: 2024-03-15 18:00:00 -- 1710490943118765 --
// 相同位数最大的数字10进制=9999999999999999 => 2QGPCKVNG1R, 长度是11位
// 如果是10位36进制数字ZZZZZZZZZZ对应的10进制数是3656158440062975, 这个数字表示的时间是2085-11-09 23:34:00
// 因此到2085年，时间分配10位才会溢出
func GenId(id ...string) string {
	if len(id) != 0 && id[0] != "" {
		return id[0]
	}
	v := [16]byte{}
	copy(v[0:10], strconv.FormatInt(time.Now().UnixMicro(), 36))
	copy(v[10:13], ipNum)
	copy(v[13:16], strconv.FormatInt(source.Int63n(46655), 36))
	return strings.ToUpper(string(v[:]))
}

func GenId2(id ...string) string {
	if len(id) != 0 && id[0] != "" {
		return id[0]
	}
	s := uint64(time.Now().UnixMicro()*1000 + source.Int63n(1000)) // % 4738381338321616895
	return strings.ToUpper(strconv.FormatUint(s, 36))
}
