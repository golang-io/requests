package requests

import (
	"net/http/pprof"
)

// Pprof 挂载 net/http/pprof（与标准库 init 注册方式一致）。
// Index 用尾斜杠 multi 覆盖 /heap 等；cmdline/profile/symbol/trace 精确注册优先。
// 生产环境慎用；block/mutex 需自行 SetBlockProfileRate / SetMutexProfileFraction。
//
//	mux.Pprof()
//	// go tool pprof http://localhost:8080/debug/pprof/heap
func (mux *ServeMux) Pprof() {
	mux.Route("/debug/pprof/", pprof.Index)
	mux.Route("/debug/pprof/cmdline", pprof.Cmdline)
	mux.Route("/debug/pprof/profile", pprof.Profile)
	mux.Route("/debug/pprof/symbol", pprof.Symbol)
	mux.Route("/debug/pprof/trace", pprof.Trace)
}
