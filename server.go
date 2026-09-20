package requests

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ErrHandler 是默认的错误处理器，返回一个简单的错误响应
// ErrHandler is the default error handler that returns a simple error response
//
// 参数 / Parameters:
//   - err: 错误消息 / Error message
//   - code: HTTP 状态码 / HTTP status code
//
// 返回值 / Returns:
//   - http.Handler: 错误处理器 / Error handler
var ErrHandler = func(err string, code int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, err, code)
	})
}

// WarpHandler 包装一个 http.Handler
// WarpHandler wraps an http.Handler
//
// 参数 / Parameters:
//   - next: 下一个处理器 / Next handler
//
// 返回值 / Returns:
//   - func(http.Handler) http.Handler: 中间件函数 / Middleware function
//
// 说明 / Notes:
//   - 这是一个装饰器模式的实现
//   - This is an implementation of the decorator pattern
func WarpHandler(next http.Handler) func(http.Handler) http.Handler {
	return func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}

// validParamName 校验通配参数名（字母/数字/下划线，且非空）
// validParamName validates a wildcard name (letters/digits/underscore, non-empty)
func validParamName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

// isParam 判断路径段是否为单段参数（:id 或 {id}），并提取参数名
// isParam reports whether a path segment is a single-segment parameter (:id or {id})
//
// 支持的语法 / Supported syntaxes:
//   - :id 风格（兼容 Gin、Echo 等框架）/ :id style (compatible with Gin, Echo, etc.)
//   - {id} 风格（兼容 Go 1.22+ 标准库）/ {id} style (compatible with Go 1.22+ standard library)
//
// 示例 / Example:
//
//	isParam, name := isParam(":id")      // true, "id"
//	isParam, name := isParam("{id}")     // true, "id"
//	isParam, name := isParam("users")    // false, ""
func isParam(segment string) (bool, string) {
	var name string
	switch {
	case len(segment) > 1 && segment[0] == ':':
		name = segment[1:]
		if strings.HasSuffix(name, "...") {
			return false, ""
		}
	case len(segment) > 2 && segment[0] == '{' && segment[len(segment)-1] == '}':
		name = segment[1 : len(segment)-1]
		if strings.HasSuffix(name, "...") {
			return false, ""
		}
	default:
		return false, ""
	}
	if !validParamName(name) {
		return false, ""
	}
	return true, name
}

// isMultiParam 判断是否为多段通配 {name...} 或 :name...（必须位于模式末尾）
// isMultiParam reports whether the segment is a multi wildcard {name...} or :name... (must be last)
func isMultiParam(segment string) (bool, string) {
	var name string
	switch {
	case strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "...}"):
		name = segment[1 : len(segment)-4]
	case strings.HasPrefix(segment, ":") && strings.HasSuffix(segment, "..."):
		name = segment[1 : len(segment)-3]
	default:
		return false, ""
	}
	if !validParamName(name) {
		return false, ""
	}
	return true, name
}

// ParamNode 表示一个路径参数节点
// ParamNode represents a path parameter node
//
// 用于存储路径参数信息，支持 :id 和 {id} 两种语法
// Used to store path parameter information, supports both :id and {id} syntaxes
//
// 示例 / Example:
//
//	路由: /api/users/:id
//	参数节点: name="id", node=子节点树
//	Route: /api/users/:id
//	ParamNode: name="id", node=child node tree
type ParamNode struct {
	name string // 参数名（如 "id"）/ Parameter name (e.g., "id")
	node *Node  // 参数节点对应的子节点 / Child node for this parameter
}

// Node 是路由树节点：next 字面段，param 单段通配，multi 吃剩余路径（对齐 net/http）
type Node struct {
	path      string                  // 当前路径段
	opts      []Option                // 节点级 Option
	next      map[string]*Node        // 字面子节点
	param     *ParamNode              // :id / {id}
	methods   map[string]http.Handler // method → handler
	multi     *Node                   // "..."（标准库 multiChild）
	multiName string                  // 命名 multi 的 PathValue 名；空=匿名尾斜杠
}

// NewNode 创建路由树节点
func NewNode(path string, opts ...Option) *Node {
	return &Node{
		path:    path,
		opts:    opts,
		next:    make(map[string]*Node),
		methods: make(map[string]http.Handler),
	}
}

// addChild 返回字面子节点（对齐 net/http.addChild）；不存在则创建
func (node *Node) addChild(seg string, opts []Option) *Node {
	if n, ok := node.next[seg]; ok {
		return n
	}
	n := NewNode(seg, opts...)
	node.next[seg] = n
	return n
}

// addParam 返回单段参数子节点（:id / {id}）；不存在则创建
func (node *Node) addParam(name, seg string, opts []Option) *Node {
	if node.param == nil {
		node.param = &ParamNode{name: name, node: NewNode(seg, opts...)}
	} else if node.param.name != name {
		node.param.name = name
	}
	return node.param.node
}

// addMulti 返回 multi 子节点（标准库 multiChild）；name 空=匿名尾斜杠
func (node *Node) addMulti(name string) *Node {
	if node.multi == nil {
		node.multi = NewNode("...")
	}
	node.multiName = name
	return node.multi
}

// set 登记本节点某个 method 的 handler（对齐 net/http.routingNode.set）
func (node *Node) set(method string, h http.Handler, opts []Option) {
	node.methods[method], node.opts = h, opts
}

// Add 向路由树中添加一个路由
//
//	:id / {id} 单段；/foo/ 匿名 multi；/foo/{path...} 命名 multi
//	仅有子树时，缺尾斜杠 → 307 到 path+"/"（对齐 Go 1.22+ ServeMux）
func (node *Node) Add(path string, h http.HandlerFunc, opts ...Option) {
	if path == "" {
		panic("path is empty")
	}
	options := newOptions(opts)
	method := options.Method

	if path == "/" {
		node.addMulti("").set(method, h, opts)
		return
	}

	anonMulti := strings.HasSuffix(path, "/")
	if anonMulti {
		path = strings.TrimSuffix(path, "/")
	}

	parts := strings.Split(path[1:], "/")
	current := node
	for i, p := range parts {
		if p == "" {
			continue
		}
		if ok, name := isMultiParam(p); ok {
			if i != len(parts)-1 {
				panic("requests: multi wildcard {...} must be the final path segment")
			}
			current.addMulti(name).set(method, h, opts)
			return
		}
		if ok, name := isParam(p); ok {
			current = current.addParam(name, p, opts)
			continue
		}
		current = current.addChild(p, opts)
	}

	if anonMulti {
		current.addMulti("").set(method, h, opts)
		return
	}
	current.set(method, h, opts)
}

// Find 在路由树中查找匹配的节点
// Find finds a matching node in the routing tree
//
// 匹配顺序 / Match order: 字面 → 单段参数 → multi（可回溯）/ literal → param → multi (with backtracking)
func (node *Node) Find(path string, r *http.Request) (*Node, bool) {
	return node.find(splitPathSegments(path), r)
}

// splitPathSegments 按 "/" 分割并去掉空段
// splitPathSegments splits on "/" and drops empty segments
func splitPathSegments(path string) []string {
	return strings.FieldsFunc(path, func(r rune) bool { return r == '/' })
}

// lookupNode 仅按字面/单段参数走到终点（不触发 multi），用于尾斜杠 307 判断
func (node *Node) lookupNode(segments []string) *Node {
	current := node
	for _, seg := range segments {
		if next, ok := current.next[seg]; ok {
			current = next
			continue
		}
		if current.param != nil {
			current = current.param.node
			continue
		}
		return nil
	}
	return current
}

// find 字面 → 参数 → multi（失败回溯），对齐 net/http.matchPath
func (node *Node) find(segments []string, r *http.Request) (*Node, bool) {
	if len(segments) == 0 {
		if len(node.methods) > 0 {
			return node, true
		}
		return node.matchMulti(r, "")
	}

	seg, rest := segments[0], segments[1:]

	if next, ok := node.next[seg]; ok {
		if n, ok := next.find(rest, r); ok {
			return n, true
		}
	}

	if node.param != nil {
		if n, ok := node.param.node.find(rest, r); ok {
			if r != nil {
				r.SetPathValue(node.param.name, seg)
			}
			return n, true
		}
	}

	return node.matchMulti(r, strings.Join(segments, "/"))
}

// matchMulti 尝试 multi；命名时写入 PathValue（匿名尾斜杠不写）
func (node *Node) matchMulti(r *http.Request, remainder string) (*Node, bool) {
	if node.multi == nil || len(node.multi.methods) == 0 {
		return node, false
	}
	if r != nil && node.multiName != "" {
		r.SetPathValue(node.multiName, remainder)
	}
	return node.multi, true
}

// paths 获取当前节点的所有子路径
// paths gets all sub-paths of the current node
//
// 返回值 / Returns:
//   - []string: 子路径列表 / List of sub-paths
func (node *Node) paths() []string {
	var v []string
	for k := range node.next {
		v = append(v, k)
	}
	return v
}

// Print 打印路由树结构（用于调试）
// Print prints the routing tree structure (for debugging)
//
// 参数 / Parameters:
//   - w: 输出写入器 / Output writer
//
// 示例 / Example:
//
//	mux := requests.NewServeMux()
//	mux.Route("/api/users", handler)
//	mux.Print(os.Stdout)  // 打印路由树 / Print routing tree
func (node *Node) Print(w io.Writer) {
	node.print(0, w)
}

// print 递归打印路由树（内部方法）
// print recursively prints the routing tree (internal method)
func (node *Node) print(m int, w io.Writer) {
	paths := node.paths()
	for method, handler := range node.methods {
		fmt.Fprintf(w, "%spath=%s, method=%s, handler=%v, next=%#v\n", strings.Repeat("    ", m), node.path, method, handler, paths)
	}
	if node.multi != nil {
		for method, handler := range node.multi.methods {
			fmt.Fprintf(w, "%spath=%s/..., method=%s, handler=%v\n", strings.Repeat("    ", m), node.path, method, handler)
		}
	}
	for _, p := range paths {
		node.next[p].print(m+1, w)
	}
	if node.param != nil {
		node.param.node.print(m+1, w)
	}
}

// ServeMux 是 HTTP 请求路由多路复用器
// ServeMux is an HTTP request router and multiplexer
//
// 核心特性 / Core Features:
//   - 基于前缀树（Trie）的高效路由匹配 / Efficient routing based on Trie tree
//   - 支持中间件链 / Supports middleware chain
//   - 支持路径级别的配置 / Supports path-level configuration
//   - 兼容 net/http.ServeMux / Compatible with net/http.ServeMux
//   - 支持所有 HTTP 方法 / Supports all HTTP methods
//
// 设计模式 / Design Pattern:
//   - 责任链模式（Middleware）/ Chain of Responsibility (Middleware)
//   - 组合模式（Node Tree）/ Composite Pattern (Node Tree)
//
// 示例 / Example:
//
//	mux := requests.NewServeMux(
//	    requests.URL("0.0.0.0:8080"),
//	    requests.Use(loggingMiddleware),
//	)
//	mux.Route("/", homeHandler)
//	mux.GET("/users", getUsersHandler)
//	mux.POST("/users", createUserHandler)
//	requests.ListenAndServe(context.Background(), mux)
type ServeMux struct {
	opts []Option // 路由器级别的配置选项 / Router-level configuration options
	root *Node    // 路由树的根节点 / Root node of the routing tree
}

// NewServeMux 创建一个新的路由多路复用器
// NewServeMux creates a new HTTP request router
//
// 参数 / Parameters:
//   - opts: 配置选项（会话级别）/ Configuration options (session-level)
//
// 返回值 / Returns:
//   - *ServeMux: 路由器实例 / Router instance
//
// 示例 / Example:
//
//	// 基础路由器
//	// Basic router
//	mux := requests.NewServeMux()
//
//	// 带中间件的路由器
//	// Router with middleware
//	mux := requests.NewServeMux(
//	    requests.URL("0.0.0.0:8080"),
//	    requests.Use(loggingMiddleware, authMiddleware),
//	)
func NewServeMux(opts ...Option) *ServeMux {
	return &ServeMux{
		opts: opts,
		root: NewNode("/"),
	}
}

// Print 打印路由树结构（用于调试）
// Print prints the routing tree structure (for debugging)
func (mux *ServeMux) Print(w io.Writer) {
	mux.root.Print(w)
}

// HandleFunc 注册一个处理函数到指定路径
// HandleFunc registers a handler function for the given path
//
// 参数 / Parameters:
//   - path: 路由路径 / Route path
//   - f: 处理函数 / Handler function
//   - opts: 配置选项（可指定HTTP方法）/ Configuration options (can specify HTTP method)
//
// 注意 / Notes:
//   - 路径不能覆盖，如果路径不工作，可能是已经存在
//   - Paths cannot be overridden; if a path doesn't work, it may already exist
//
// 示例 / Example:
//
//	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
//	    fmt.Fprintf(w, "Users list")
//	}, requests.Method("GET"))
func (mux *ServeMux) HandleFunc(path string, f func(http.ResponseWriter, *http.Request), opts ...Option) {
	mux.root.Add(path, f, opts...)
}

// Handle 注册一个 http.Handler 到指定路径
// Handle registers an http.Handler for the given path
//
// 参数 / Parameters:
//   - path: 路由路径 / Route path
//   - h: HTTP处理器 / HTTP handler
//   - opts: 配置选项 / Configuration options
//
// 示例 / Example:
//
//	mux.Handle("/static", http.FileServer(http.Dir("./public")))
func (mux *ServeMux) Handle(path string, h http.Handler, opts ...Option) {
	mux.root.Add(path, h.ServeHTTP, opts...)
}

// Route 注册任意类型的处理器到指定路径
// Route registers any type of handler for the given path
//
// 参数 / Parameters:
//   - path: 路由路径 / Route path
//   - v: 处理器（支持多种类型）/ Handler (supports multiple types)
//   - opts: 配置选项 / Configuration options
//
// 支持的处理器类型 / Supported Handler Types:
//   - http.HandlerFunc
//   - http.Handler
//   - func(http.ResponseWriter, *http.Request)
//
// 示例 / Example:
//
//	mux.Route("/", func(w http.ResponseWriter, r *http.Request) {
//	    fmt.Fprintf(w, "Home")
//	})
//
//	mux.Route("/static", http.FileServer(http.Dir("./public")))
func (mux *ServeMux) Route(path string, v any, opts ...Option) {
	switch h := v.(type) {
	case http.HandlerFunc:
		mux.HandleFunc(path, h, opts...)
	case http.Handler:
		mux.Handle(path, h, opts...)
	case func(http.ResponseWriter, *http.Request):
		mux.HandleFunc(path, h, opts...)
	default:
		panic("unknown handler type")
	}
}

// GET 注册一个 GET 请求处理器
// GET registers a handler for GET requests
//
// 参数 / Parameters:
//   - path: 路由路径 / Route path
//   - v: 处理器 / Handler
//   - opts: 配置选项 / Configuration options
//
// 示例 / Example:
//
//	mux.GET("/api/users", getUsersHandler)
func (mux *ServeMux) GET(path string, v any, opts ...Option) {
	mux.Route(path, v, append(opts, Method("GET"))...)
}

// POST 注册一个 POST 请求处理器
// POST registers a handler for POST requests
//
// 示例 / Example:
//
//	mux.POST("/api/users", createUserHandler)
func (mux *ServeMux) POST(path string, v any, opts ...Option) {
	mux.Route(path, v, append(opts, Method("POST"))...)
}

// PUT 注册一个 PUT 请求处理器
// PUT registers a handler for PUT requests
//
// 示例 / Example:
//
//	mux.PUT("/api/users/:id", updateUserHandler)
func (mux *ServeMux) PUT(path string, v any, opts ...Option) {
	mux.Route(path, v, append(opts, Method("PUT"))...)
}

// DELETE 注册一个 DELETE 请求处理器
// DELETE registers a handler for DELETE requests
//
// 示例 / Example:
//
//	mux.DELETE("/api/users/:id", deleteUserHandler)
func (mux *ServeMux) DELETE(path string, v any, opts ...Option) {
	mux.Route(path, v, append(opts, Method("DELETE"))...)
}

// OPTIONS 注册一个 OPTIONS 请求处理器
// OPTIONS registers a handler for OPTIONS requests
func (mux *ServeMux) OPTIONS(path string, v any, opts ...Option) {
	mux.Route(path, v, append(opts, Method("OPTIONS"))...)
}

// HEAD 注册一个 HEAD 请求处理器
// HEAD registers a handler for HEAD requests
func (mux *ServeMux) HEAD(path string, v any, opts ...Option) {
	mux.Route(path, v, append(opts, Method("HEAD"))...)
}

// CONNECT 注册一个 CONNECT 请求处理器
// CONNECT registers a handler for CONNECT requests
func (mux *ServeMux) CONNECT(path string, v any, opts ...Option) {
	mux.Route(path, v, append(opts, Method("CONNECT"))...)
}

// TRACE 注册一个 TRACE 请求处理器
// TRACE registers a handler for TRACE requests
func (mux *ServeMux) TRACE(path string, v any, opts ...Option) {
	mux.Route(path, v, append(opts, Method("TRACE"))...)
}

// Redirect 设置路径重定向
// Redirect sets up a redirect from source to target path
//
// 参数 / Parameters:
//   - source: 源路径 / Source path
//   - target: 目标路径 / Target path
//
// 说明 / Notes:
//   - 使用 301 永久重定向 / Uses 301 Moved Permanently
//
// 示例 / Example:
//
//	mux.Redirect("/old-path", "/new-path")
func (mux *ServeMux) Redirect(source, target string) {
	mux.Route(source, http.RedirectHandler(target, http.StatusMovedPermanently).ServeHTTP)
}

// Use 注册全局中间件（兼容 net/http.ServeMux）
// Use registers global middleware (compatible with net/http.ServeMux)
//
// 参数 / Parameters:
//   - fn: 中间件函数列表 / Middleware function list
//
// 示例 / Example:
//
//	mux.Use(loggingMiddleware, authMiddleware, corsMiddleware)
func (mux *ServeMux) Use(fn ...func(http.Handler) http.Handler) {
	mux.opts = append(mux.opts, Use(fn...))
}

// ServeHTTP 实现 http.Handler 接口
// ServeHTTP implements the http.Handler interface
//
// 处理流程 / Processing Flow:
//  1. 路由匹配：在路由树中查找最长匹配 / Route matching: find longest match in routing tree
//  2. 路由校验：不存在则返回 404 / Route validation: return 404 if not exists
//  3. 方法校验：方法不支持则返回 405 / Method validation: return 405 if method not supported
//  4. 中间件链：按注册顺序应用中间件 / Middleware chain: apply middleware in registration order
//  5. 执行处理器：调用最终的处理函数 / Execute handler: call final handler function
//
// 参数 / Parameters:
//   - w: 响应写入器 / Response writer
//   - r: HTTP 请求 / HTTP request
//
// 示例 / Example:
//
//	mux := requests.NewServeMux()
//	mux.Route("/api/users", handler)
//	http.ListenAndServe(":8080", mux)  // mux 实现了 http.Handler
func (mux *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Path
	segs := splitPathSegments(rawPath)

	// 尾斜杠 307：仅有 multi、无精确 handler、请求无尾斜杠 → path+"/"（对齐 net/http）
	if rawPath != "/" && !strings.HasSuffix(rawPath, "/") {
		if n := mux.root.lookupNode(segs); n != nil && len(n.methods) == 0 && n.multi != nil && len(n.multi.methods) > 0 {
			dest := rawPath + "/"
			if q := r.URL.RawQuery; q != "" {
				dest += "?" + q
			}
			http.Redirect(w, r, dest, http.StatusTemporaryRedirect)
			return
		}
	}

	current, matched := mux.root.find(segs, r)
	options := newOptions(mux.opts, current.opts...)

	handler := mux.pickHandler(current, matched, r.Method)
	for _, h := range options.HttpHandler {
		handler = h(handler)
	}
	handler.ServeHTTP(w, r)
}

// pickHandler 按匹配结果与方法选择 404 / 405 / 业务 handler
// pickHandler selects 404 / 405 / business handler from match result and method
func (mux *ServeMux) pickHandler(node *Node, matched bool, method string) http.Handler {
	if !matched || len(node.methods) == 0 {
		return ErrHandler(http.StatusText(http.StatusNotFound), http.StatusNotFound)
	}
	if h := node.methods[method]; h != nil {
		return h
	}
	if h := node.methods[""]; h != nil {
		return h
	}
	return ErrHandler(http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

// Server 是 HTTP 服务器封装
// Server is an HTTP server wrapper
//
// 功能 / Features:
//   - 优雅关闭 / Graceful shutdown
//   - 支持 HTTP 和 HTTPS / Supports HTTP and HTTPS
//   - 可配置超时 / Configurable timeouts
//   - 生命周期回调 / Lifecycle callbacks
//
// 示例 / Example:
//
//	mux := requests.NewServeMux()
//	server := requests.NewServer(ctx, mux,
//	    requests.URL("0.0.0.0:8080"),
//	    requests.OnStart(func(s *http.Server) {
//	        log.Printf("Server started on %s", s.Addr)
//	    }),
//	)
//	server.ListenAndServe()
type Server struct {
	options Options      // 服务器配置选项 / Server configuration options
	server  *http.Server // 底层 HTTP 服务器 / Underlying HTTP server
}

// NewServer 创建一个新的 HTTP 服务器
// NewServer creates a new HTTP server
//
// 参数 / Parameters:
//   - ctx: 上下文（用于优雅关闭）/ Context (for graceful shutdown)
//   - h: HTTP 处理器 / HTTP handler
//   - opts: 配置选项（不会添加到 ServeMux）/ Configuration options (not added to ServeMux)
//
// 返回值 / Returns:
//   - *Server: 服务器实例 / Server instance
//
// 注意 / Notes:
//   - 会自动在 ctx.Done() 时优雅关闭服务器
//   - Will automatically shutdown gracefully when ctx.Done()
//   - opts 不会传递给 ServeMux，仅用于服务器配置
//   - opts are not passed to ServeMux, only for server configuration
//
// 示例 / Example:
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	mux := requests.NewServeMux()
//	server := requests.NewServer(ctx, mux,
//	    requests.URL("0.0.0.0:8080"),
//	    requests.Timeout(30*time.Second),
//	)
//	server.ListenAndServe()
func NewServer(ctx context.Context, h http.Handler, opts ...Option) *Server {
	s := &Server{server: &http.Server{Handler: h}}

	// 尝试获取 ServeMux 的配置
	// Try to get ServeMux configuration
	mux, ok := h.(*ServeMux)
	if !ok {
		mux = NewServeMux()
	}

	// 合并配置选项
	// Merge configuration options
	s.options = newOptions(mux.opts, opts...)

	// 设置超时
	// Set timeouts
	s.server.ReadTimeout, s.server.WriteTimeout = s.options.Timeout, s.options.Timeout

	// 解析监听地址
	// Parse listen address
	u, err := url.Parse(s.options.URL)
	if err != nil {
		panic(err)
	}

	// 设置服务器地址
	// Set server address
	s.server.Addr = u.Host

	// 调用启动回调
	// Call start callback
	s.options.OnStart(s.server)

	// 注册关闭回调
	// Register shutdown callback
	s.server.RegisterOnShutdown(func() { s.options.OnShutdown(s.server) })

	// 启动优雅关闭监听器
	// Start graceful shutdown listener
	go s.Shutdown(ctx)

	return s
}

// Shutdown 优雅地关闭服务器，不中断活动连接
// Shutdown gracefully shuts down the server without interrupting any active connections
//
// 参数 / Parameters:
//   - ctx: 上下文 / Context
//
// 返回值 / Returns:
//   - error: 关闭过程中的错误 / Error during shutdown
//
// 说明 / Notes:
//   - 会等待 ctx.Done() 信号 / Waits for ctx.Done() signal
//   - 不会强制中断正在处理的请求 / Won't forcefully interrupt active requests
//
// 示例 / Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//	server.Shutdown(ctx)
func (s *Server) Shutdown(ctx context.Context) error {
	// 等待上下文取消信号
	// Wait for context cancellation signal
	<-ctx.Done()
	return s.server.Shutdown(ctx)
}

// ListenAndServe 启动 HTTP 或 HTTPS 服务器并监听请求
// ListenAndServe starts the HTTP or HTTPS server and listens for requests
//
// 功能说明 / Functionality:
//   - 在 TCP 网络地址 srv.Addr 上监听 / Listens on TCP network address srv.Addr
//   - 根据是否配置证书自动选择 HTTP 或 HTTPS / Automatically selects HTTP or HTTPS based on cert configuration
//   - 启用 TCP keep-alive / Enables TCP keep-alives for accepted connections
//   - 阻塞直到服务器关闭 / Blocks until server shutdown
//
// HTTP vs HTTPS:
//   - 如果设置了 certFile 和 keyFile，启动 HTTPS 服务器
//   - If certFile and keyFile are set, starts HTTPS server
//   - 否则启动 HTTP 服务器
//   - Otherwise starts HTTP server
//
// 返回值 / Returns:
//   - error: 总是返回非 nil 错误，服务器关闭后返回 ErrServerClosed
//   - error: Always returns a non-nil error; returns ErrServerClosed after shutdown
//
// 示例 / Example:
//
//	// HTTP 服务器
//	// HTTP server
//	mux := requests.NewServeMux(requests.URL("0.0.0.0:8080"))
//	server := requests.NewServer(ctx, mux)
//	if err := server.ListenAndServe(); err != nil {
//	    log.Fatal(err)
//	}
//
//	// HTTPS 服务器
//	// HTTPS server
//	mux := requests.NewServeMux(
//	    requests.URL("0.0.0.0:443"),
//	    requests.CertKey("cert.pem", "key.pem"),
//	)
//	server := requests.NewServer(ctx, mux)
//	if err := server.ListenAndServe(); err != nil {
//	    log.Fatal(err)
//	}
func (s *Server) ListenAndServe() (err error) {
	// 根据是否配置证书选择 HTTP 或 HTTPS
	// Choose HTTP or HTTPS based on cert configuration
	if s.options.certFile == "" || s.options.keyFile == "" {
		// 启动 HTTP 服务器 / Start HTTP server
		return s.server.ListenAndServe()
	}
	// 启动 HTTPS 服务器 / Start HTTPS server
	return s.server.ListenAndServeTLS(s.options.certFile, s.options.keyFile)
}

// ListenAndServe 是一个便捷函数，创建服务器并启动监听
// ListenAndServe is a convenience function that creates a server and starts listening
//
// 参数 / Parameters:
//   - ctx: 上下文（用于优雅关闭）/ Context (for graceful shutdown)
//   - h: HTTP 处理器 / HTTP handler
//   - opts: 配置选项 / Configuration options
//
// 返回值 / Returns:
//   - error: 服务器错误 / Server error
//
// 说明 / Notes:
//   - 这是一个阻塞调用，会一直运行直到服务器关闭
//   - This is a blocking call that runs until server shutdown
//   - 当 ctx 被取消时，服务器会优雅关闭
//   - Server shuts down gracefully when ctx is cancelled
//
// 示例 / Example:
//
//	// 简单服务器
//	// Simple server
//	mux := requests.NewServeMux()
//	mux.Route("/", homeHandler)
//	err := requests.ListenAndServe(
//	    context.Background(),
//	    mux,
//	    requests.URL("0.0.0.0:8080"),
//	)
//
//	// 带优雅关闭的服务器
//	// Server with graceful shutdown
//	ctx, cancel := context.WithCancel(context.Background())
//	go func() {
//	    <-sigint  // 等待中断信号 / Wait for interrupt signal
//	    cancel()  // 触发优雅关闭 / Trigger graceful shutdown
//	}()
//	requests.ListenAndServe(ctx, mux, requests.URL("0.0.0.0:8080"))
func ListenAndServe(ctx context.Context, h http.Handler, opts ...Option) error {
	s := NewServer(ctx, h, opts...)
	return s.ListenAndServe()
}
