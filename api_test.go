package requests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func setupTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Method", r.Method)
		w.Header().Set("X-Request-URL", r.URL.String())

		if r.Method == "HEAD" {
			w.WriteHeader(http.StatusOK)
			return
		}

		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"method":"` + r.Method + `","url":"` + r.URL.String() + `","body":"` + string(body) + `"}`))
	}))
}

func TestGet(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	resp, err := Get(server.URL + "/get?param=value")
	if err != nil {
		t.Fatalf("Get 请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", resp.StatusCode)
	}

	if method := resp.Header.Get("X-Request-Method"); method != "GET" {
		t.Errorf("期望请求方法为 GET，实际为 %s", method)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应体失败: %v", err)
	}

	if !strings.Contains(string(body), `"method":"GET"`) {
		t.Errorf("响应体中应包含请求方法 GET，实际为: %s", string(body))
	}
}

func TestPost(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	body := strings.NewReader(`{"key":"value"}`)
	resp, err := Post(server.URL+"/post", "application/json", body)
	if err != nil {
		t.Fatalf("Post 请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", resp.StatusCode)
	}

	if method := resp.Header.Get("X-Request-Method"); method != "POST" {
		t.Errorf("期望请求方法为 POST，实际为 %s", method)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应体失败: %v", err)
	}

	if !strings.Contains(string(respBody), `"method":"POST"`) {
		t.Errorf("响应体中应包含请求方法 POST，实际为: %s", string(respBody))
	}

	if !strings.Contains(string(respBody), `"body":"{"key":"value"}"`) {
		t.Errorf("响应体中应包含请求体，实际为: %s", string(respBody))
	}
}

func TestPUT(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	body := strings.NewReader(`{"key":"updated"}`)
	resp, err := Put(server.URL+"/put", "application/json", body)
	if err != nil {
		t.Fatalf("PUT 请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", resp.StatusCode)
	}

	if method := resp.Header.Get("X-Request-Method"); method != "PUT" {
		t.Errorf("期望请求方法为 PUT，实际为 %s", method)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应体失败: %v", err)
	}

	if !strings.Contains(string(respBody), `"method":"PUT"`) {
		t.Errorf("响应体中应包含请求方法 PUT，实际为: %s", string(respBody))
	}
}

func TestDelete(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	body := strings.NewReader(`{"id":123}`)
	resp, err := Delete(server.URL+"/delete", "application/json", body)
	if err != nil {
		t.Fatalf("Delete 请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", resp.StatusCode)
	}

	if method := resp.Header.Get("X-Request-Method"); method != "DELETE" {
		t.Errorf("期望请求方法为 DELETE，实际为 %s", method)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应体失败: %v", err)
	}

	if !strings.Contains(string(respBody), `"method":"DELETE"`) {
		t.Errorf("响应体中应包含请求方法 DELETE，实际为: %s", string(respBody))
	}
}

func TestHead(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	resp, err := Head(server.URL + "/head")
	if err != nil {
		t.Fatalf("Head 请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", resp.StatusCode)
	}

	if method := resp.Header.Get("X-Request-Method"); method != "HEAD" {
		t.Errorf("期望请求方法为 HEAD，实际为 %s", method)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应体失败: %v", err)
	}

	if len(body) > 0 {
		t.Errorf("HEAD 请求不应返回响应体，实际返回: %s", string(body))
	}
}

func TestPostForm(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	form := url.Values{}
	form.Add("username", "testuser")
	form.Add("password", "testpass")

	resp, err := PostForm(server.URL+"/form", form)
	if err != nil {
		t.Fatalf("PostForm 请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", resp.StatusCode)
	}

	if method := resp.Header.Get("X-Request-Method"); method != "POST" {
		t.Errorf("期望请求方法为 POST，实际为 %s", method)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应体失败: %v", err)
	}

	expectedFormData := `{"method":"POST","url":"/form","body":"password=testpass&username=testuser"}`
	if !strings.Contains(string(respBody), expectedFormData) {
		t.Errorf("响应体中应包含表单数据 %s，实际为: %s", expectedFormData, string(respBody))
	}
}
