package requests

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/pprof"
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

// isParam 判断路径段是否为参数，并提取参数名
// isParam determines if a path segment is a parameter and extracts the parameter name
//
// 支持的语法 / Supported syntaxes:
//   - :id 风格（兼容 Gin、Echo 等框架）/ :id style (compatible with Gin, Echo, etc.)
//   - {id} 风格（兼容 Go 1.22+ 标准库）/ {id} style (compatible with Go 1.22+ standard library)
//
// 参数 / Parameters:
//   - segment: 路径段 / Path segment
//
// 返回值 / Returns:
//   - bool: 是否为参数 / Whether it is a parameter
//   - string: 参数名（如果不是参数则返回空字符串）/ Parameter name (empty string if not a parameter)
//
// 示例 / Example:
//
//	isParam, name := isParam(":id")      // true, "id"
//	isParam, name := isParam("{id}")     // true, "id"
//	isParam, name := isParam("users")    // false, ""
func isParam(segment string) (bool, string) {
	if segment == "" {
		return false, ""
	}

	// 检查 :id 语法（以冒号开头，且长度大于1）
	// Check :id syntax (starts with colon, length > 1)
	if strings.HasPrefix(segment, ":") && len(segment) > 1 {
		// 验证参数名只包含字母、数字、下划线
		// Validate parameter name contains only letters, numbers, underscores
		name := segment[1:]
		for _, r := range name {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
				return false, ""
			}
		}
		return true, name
	}

	// 检查 {id} 语法（以 { 开头，以 } 结尾，且长度大于2）
	// Check {id} syntax (starts with {, ends with }, length > 2)
	if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") && len(segment) > 2 {
		// 验证参数名只包含字母、数字、下划线
		// Validate parameter name contains only letters, numbers, underscores
		name := segment[1 : len(segment)-1]
		for _, r := range name {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
				return false, ""
			}
		}
		return true, name
	}

	return false, ""
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

// Node 是路由树（Trie树）的节点
// Node is a node in the routing tree (Trie tree)
//
// 结构说明 / Structure:
//   - 使用前缀树（Trie）实现高效的路由匹配
//   - Uses a Trie tree for efficient route matching
//   - 支持最长前缀匹配原则
//   - Supports longest prefix matching
//   - 每个节点可以有多个 HTTP 方法的处理器
//   - Each node can have handlers for multiple HTTP methods
//   - 支持路径参数（通过 param 字段）
//   - Supports path parameters (via param field)
//
// 示例 / Example:
//
//	路由: /api/users/123/profile
//	树结构: / -> api -> users -> 123 -> profile
//	Route: /api/users/123/profile
//	Tree: / -> api -> users -> 123 -> profile
//
//	路由: /api/users/:id
//	树结构: / -> api -> users -> param(name="id") -> ...
//	Route: /api/users/:id
//	Tree: / -> api -> users -> param(name="id") -> ...
type Node struct {
	path    string                  // 当前节点的路径片段 / Current node's path segment
	opts    []Option                // 节点的配置选项 / Node's configuration options
	next    map[string]*Node        // 子节点映射（精确匹配）/ Child nodes map (exact match)
	param   *ParamNode              // 参数节点（用于 :id 或 {id}）/ Parameter node (for :id or {id})
	methods map[string]http.Handler // HTTP 方法到处理器的映射 / HTTP method to handler mapping
}

// NewNode 创建一个新的路由树节点
// NewNode creates a new routing tree node
//
// 参数 / Parameters:
//   - path: 路径片段 / Path segment
//   - h: 默认处理器 / Default handler
//   - opts: 配置选项 / Configuration options
//
// 返回值 / Returns:
//   - *Node: 新创建的节点 / Newly created node
func NewNode(path string, h http.Handler, opts ...Option) *Node {
	return &Node{
		path:    path,
		opts:    opts,
		next:    make(map[string]*Node),
		param:   nil,
		methods: make(map[string]http.Handler),
	}
}

// Add 向路由树中添加一个路由
// Add adds a route to the routing tree
//
// 参数 / Parameters:
//   - path: 路由路径（如 "/api/users" 或 "/api/users/:id"）/ Route path (e.g., "/api/users" or "/api/users/:id")
//   - h: 处理函数 / Handler function
//   - opts: 配置选项（可以指定 HTTP 方法）/ Configuration options (can specify HTTP method)
//
// 实现原理 / Implementation:
//   - 按照 "/" 分割路径 / Split path by "/"
//   - 逐级创建树节点 / Create tree nodes level by level
//   - 支持路径参数（:id 或 {id} 语法）/ Supports path parameters (:id or {id} syntax)
//   - 支持路径覆盖（后注册的会覆盖先注册的）/ Supports path override (later registration overrides earlier)
//
// 路径参数语法 / Path parameter syntax:
//   - :id 风格（兼容 Gin、Echo 等框架）/ :id style (compatible with Gin, Echo, etc.)
//   - {id} 风格（兼容 Go 1.22+ 标准库）/ {id} style (compatible with Go 1.22+ standard library)
//
// 示例 / Example:
//
//	node.Add("/api/users", handler, requests.Method("GET"))
//	node.Add("/api/users/:id", handler, requests.Method("POST"))
//	node.Add("/api/users/{id}", handler, requests.Method("PUT"))
func (node *Node) Add(path string, h http.HandlerFunc, opts ...Option) {
	if path == "" {
		panic("path is empty")
	}

	options := newOptions(opts)

	// 处理根路径 / Handle root path
	if path == "/" {
		node.methods[options.Method], node.opts = h, opts
		return
	}

	// 逐级创建节点 / Create nodes level by level
	current := node
	for _, p := range strings.Split(path[1:], "/") {
		// 检查是否为参数段 / Check if it's a parameter segment
		if isParam, paramName := isParam(p); isParam {
			// 创建或更新参数节点 / Create or update parameter node
			if current.param == nil {
				current.param = &ParamNode{
					name: paramName,
					node: NewNode(p, http.NotFoundHandler(), opts...),
				}
			} else {
				// 如果参数节点已存在，更新参数名（如果不同）
				// If parameter node exists, update parameter name (if different)
				if current.param.name != paramName {
					current.param.name = paramName
				}
			}
			current = current.param.node
		} else {
			// 普通路径段，使用精确匹配 / Regular path segment, use exact match
			if _, ok := current.next[p]; !ok {
				current.next[p] = NewNode(p, http.NotFoundHandler(), opts...)
			}
			current = current.next[p]
		}
	}
	current.methods[options.Method], current.opts = h, opts
}

// Find 在路由树中查找匹配的节点（按照最长匹配原则）
// Find finds a matching node in the routing tree (using longest prefix matching)
//
// 参数 / Parameters:
//   - path: 要查找的路径 / Path to find
//   - r: HTTP 请求（用于设置路径参数值）/ HTTP request (for setting path parameter values)
//
// 返回值 / Returns:
//   - *Node: 匹配的节点 / Matched node
//   - bool: 是否完全匹配（所有路径段都匹配）/ Whether it's a complete match (all path segments matched)
//
// 匹配原则 / Matching Rules:
//   - 精确匹配优先于参数匹配 / Exact match takes priority over parameter match
//   - 静态路径优先于参数路径 / Static paths take priority over parameter paths
//   - 如果匹配到参数，会自动调用 r.SetPathValue() 设置参数值
//   - If a parameter is matched, r.SetPathValue() is automatically called to set the parameter value
//
// 示例 / Example:
//
//	node, matched := node.Find("/api/users/123", r)
//	if matched {
//	    // 完全匹配，可以使用 r.PathValue("id") 获取参数值
//	    // Complete match, can use r.PathValue("id") to get parameter value
//	}
func (node *Node) Find(path string, r *http.Request) (*Node, bool) {
	current := node
	segments := strings.Split(path, "/")

	// 过滤空字符串（Split 会在开头和结尾产生空字符串）
	// Filter empty strings (Split produces empty strings at start and end)
	var cleanSegments []string
	for _, seg := range segments {
		if seg != "" {
			cleanSegments = append(cleanSegments, seg)
		}
	}

	// 如果路径为空，直接返回当前节点
	// If path is empty, return current node directly
	if len(cleanSegments) == 0 {
		return current, len(current.methods) > 0
	}

	// 遍历路径段进行匹配
	// Traverse path segments for matching
	for _, seg := range cleanSegments {
		// 优先尝试精确匹配 / Try exact match first
		if next, ok := current.next[seg]; ok {
			current = next
			continue
		}

		// 精确匹配失败，尝试参数匹配 / Exact match failed, try parameter match
		if current.param != nil {
			// 使用参数节点匹配，并设置参数值
			// Use parameter node to match and set parameter value
			if r != nil {
				r.SetPathValue(current.param.name, seg)
			}
			current = current.param.node
			continue
		}

		// 既没有精确匹配也没有参数匹配，返回当前最长匹配的节点
		// Neither exact match nor parameter match, return current longest matched node
		return current, false
	}

	// 所有路径段都匹配成功，检查是否有处理器
	// All path segments matched successfully, check if there's a handler
	return current, len(current.methods) > 0
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
	for _, p := range paths {
		node.next[p].print(m+1, w)
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
		root: NewNode("/", http.NotFoundHandler()),
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
	// 1. 在路由树中查找匹配的节点（传入 r 用于设置路径参数）
	// 1. Find matching node in routing tree (pass r for setting path parameters)
	requestPath := strings.TrimLeft(r.URL.Path, "/")
	current, matched := mux.root.Find(requestPath, r)
	options := newOptions(mux.opts, current.opts...)

	// 2. 选择合适的处理器
	// 2. Select appropriate handler
	var handler http.Handler
	if !matched || len(current.methods) == 0 {
		// 路由不存在或未完全匹配，返回 404
		// Route doesn't exist or not fully matched, return 404
		handler = ErrHandler(http.StatusText(http.StatusNotFound), http.StatusNotFound)
	} else {
		// 查找方法对应的处理器
		// Find handler for the method
		if handler = current.methods[r.Method]; handler == nil {
			// 尝试使用默认处理器（空方法名）
			// Try default handler (empty method name)
			if handler = current.methods[""]; handler == nil {
				// 方法不允许，返回 405
				// Method not allowed, return 405
				handler = ErrHandler(http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			}
		}
	}

	// 3. 应用中间件链（装饰器模式）
	// 3. Apply middleware chain (decorator pattern)
	for _, h := range options.HttpHandler {
		handler = h(handler)
	}

	// 4. 执行最终的处理器
	// 4. Execute final handler
	handler.ServeHTTP(w, r)
}

// Pprof 启用性能分析接口（用于调试）
// Pprof enables performance profiling endpoints (for debugging)
//
// 说明 / Notes:
//   - 必须访问 /debug/pprof/ 路径 / Must access /debug/pprof/ path
//   - 生产环境慎用 / Use with caution in production
//
// 可用的分析接口 / Available Profiling Endpoints:
//   - /debug/pprof/ - 主页 / Home page
//   - /debug/pprof/cmdline - 命令行参数 / Command line arguments
//   - /debug/pprof/profile - CPU性能分析 / CPU profiling
//   - /debug/pprof/symbol - 符号表 / Symbol table
//   - /debug/pprof/trace - 执行追踪 / Execution trace
//
// 示例 / Example:
//
//	mux := requests.NewServeMux()
//	mux.Pprof()  // 启用性能分析 / Enable profiling
//	requests.ListenAndServe(context.Background(), mux)
//	// 访问: http://localhost:8080/debug/pprof/
func (mux *ServeMux) Pprof() {
	mux.Route("/debug/pprof", pprof.Index)
	mux.Route("/debug/pprof/cmdline", pprof.Cmdline)
	mux.Route("/debug/pprof/profile", pprof.Profile)
	mux.Route("/debug/pprof/symbol", pprof.Symbol)
	mux.Route("/debug/pprof/trace", pprof.Trace)
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
