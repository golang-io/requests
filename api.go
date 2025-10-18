package requests

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// session 是一个全局默认会话实例，用于包级别的便捷方法
// session is a global default session instance used for package-level convenience methods
var session = New()

// Get 发送 GET 请求
// 这是一个便捷方法，完全兼容 net/http 包的使用方式
// Get sends a GET request
// This is a convenience method, fully compatible with the net/http package usage
//
// 参数 / Parameters:
//   - url: 请求的URL地址 / The URL to request
//
// 返回值 / Returns:
//   - *http.Response: HTTP响应对象，注意：必须手动关闭 resp.Body / HTTP response object, note: resp.Body must be closed manually
//   - error: 请求过程中的错误 / Error during the request
//
// 示例 / Example:
//
//	resp, err := requests.Get("https://api.example.com/users")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer resp.Body.Close()
func Get(url string) (*http.Response, error) {
	return session.Do(context.Background(), MethodGet, URL(url))
}

// Post 发送 POST 请求
// Post sends a POST request
//
// 参数 / Parameters:
//   - url: 请求的URL地址 / The URL to request
//   - contentType: 内容类型，例如 "application/json" / Content type, e.g., "application/json"
//   - body: 请求体数据 / Request body data
//
// 返回值 / Returns:
//   - *http.Response: HTTP响应对象 / HTTP response object
//   - error: 请求过程中的错误 / Error during the request
//
// 示例 / Example:
//
//	body := strings.NewReader(`{"name": "John"}`)
//	resp, err := requests.Post("https://api.example.com/users", "application/json", body)
func Post(url string, contentType string, body io.Reader) (*http.Response, error) {
	return session.Do(context.TODO(), MethodPost, URL(url), Header("Content-Type", contentType), Body(body))
}

// Put 发送 PUT 请求
// Put sends a PUT request
//
// 参数 / Parameters:
//   - url: 请求的URL地址 / The URL to request
//   - contentType: 内容类型 / Content type
//   - body: 请求体数据 / Request body data
//
// 返回值 / Returns:
//   - *http.Response: HTTP响应对象 / HTTP response object
//   - error: 请求过程中的错误 / Error during the request
func Put(url, contentType string, body io.Reader) (*http.Response, error) {
	return session.Do(context.TODO(), Method("PUT"), URL(url), Header("Content-Type", contentType), Body(body))
}

// Delete 发送 DELETE 请求
// Delete sends a DELETE request
//
// 参数 / Parameters:
//   - url: 请求的URL地址 / The URL to request
//   - contentType: 内容类型 / Content type
//   - body: 请求体数据 / Request body data
//
// 返回值 / Returns:
//   - *http.Response: HTTP响应对象 / HTTP response object
//   - error: 请求过程中的错误 / Error during the request
func Delete(url, contentType string, body io.Reader) (*http.Response, error) {
	return session.Do(context.TODO(), Method("DELETE"), URL(url), Header("Content-Type", contentType), Body(body))
}

// Head 发送 HEAD 请求
// Head sends a HEAD request
//
// 参数 / Parameters:
//   - url: 请求的URL地址 / The URL to request
//
// 返回值 / Returns:
//   - resp: HTTP响应对象 / HTTP response object
//   - err: 请求过程中的错误 / Error during the request
func Head(url string) (resp *http.Response, err error) {
	return session.Do(context.Background(), Method("HEAD"), URL(url))
}

// PostForm 发送表单 POST 请求
// 自动设置 Content-Type 为 application/x-www-form-urlencoded
// PostForm sends a form POST request
// Automatically sets Content-Type to application/x-www-form-urlencoded
//
// 参数 / Parameters:
//   - url: 请求的URL地址 / The URL to request
//   - data: 表单数据 / Form data
//
// 返回值 / Returns:
//   - *http.Response: HTTP响应对象 / HTTP response object
//   - error: 请求过程中的错误 / Error during the request
//
// 示例 / Example:
//
//	data := url.Values{}
//	data.Set("username", "john")
//	data.Set("password", "secret")
//	resp, err := requests.PostForm("https://api.example.com/login", data)
func PostForm(url string, data url.Values) (*http.Response, error) {
	return session.Do(context.TODO(), MethodPost, URL(url), Header("Content-Type", "application/x-www-form-urlencoded"),
		Body(strings.NewReader(data.Encode())),
	)
}
