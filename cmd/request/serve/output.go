package serve

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// PrintVerboseRequest prints request details (curl -v style) to w.
func PrintVerboseRequest(req *http.Request, w io.Writer) {
	if req == nil {
		return
	}
	proto := req.Proto
	if proto == "" {
		proto = "HTTP/1.1"
	}
	fmt.Fprintf(w, "> %s %s %s\r\n", req.Method, req.URL.RequestURI(), proto)
	fmt.Fprintf(w, "> Host: %s\r\n", req.Host)
	for k, vv := range req.Header {
		for _, v := range vv {
			fmt.Fprintf(w, "> %s: %s\r\n", k, v)
		}
	}
	fmt.Fprintf(w, ">\r\n")
}

// PrintResponseHeaders prints response status + headers (curl -i / -v style) to w.
func PrintResponseHeaders(resp *http.Response, w io.Writer) {
	proto := resp.Proto
	if proto == "" {
		proto = "HTTP/1.1"
	}
	fmt.Fprintf(w, "%s %s\r\n", proto, resp.Status)
	for k, vv := range resp.Header {
		for _, v := range vv {
			fmt.Fprintf(w, "%s: %s\r\n", k, v)
		}
	}
	fmt.Fprintf(w, "\r\n")
}

// DumpHeaders writes response headers to a file (curl -D).
func DumpHeaders(path string, resp *http.Response) error {
	f, err := os.Create(path) // #nosec G304
	if err != nil {
		return err
	}
	defer f.Close()
	PrintResponseHeaders(resp, f)
	return nil
}

// IsFile returns true if s looks like an existing file path (not a cookie string).
func IsFile(s string) bool {
	if strings.ContainsAny(s, "=;, ") {
		return false
	}
	info, err := os.Stat(s)
	return err == nil && !info.IsDir()
}

// ParseCookieFile reads a Netscape/curl cookie file and returns a Cookie header string.
func ParseCookieFile(path string) (string, error) {
	f, err := os.Open(path) // #nosec G304
	if err != nil {
		return "", err
	}
	defer f.Close()

	var pairs []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Netscape format: domain  flag  path  secure  expiry  name  value
		fields := strings.Fields(line)
		if len(fields) >= 7 {
			pairs = append(pairs, fields[5]+"="+fields[6])
		}
	}
	return strings.Join(pairs, "; "), scanner.Err()
}
