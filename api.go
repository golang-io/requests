package requests

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var s = New()

// Get send get request
func Get(url string) (*http.Response, error) {
	return s.Do(context.Background(), MethodGet, URL(url))
}

// Post send post request
func Post(url string, contentType string, body io.Reader) (*http.Response, error) {
	return s.Do(context.TODO(), MethodPost, URL(url), Header("Content-Type", contentType), Body(body))
}

// PUT send put request
func PUT(url, contentType string, body io.Reader) (*http.Response, error) {
	return s.Do(context.TODO(), Method("PUT"), URL(url), Header("Content-Type", contentType), Body(body))
}

// Delete send delete request
func Delete(url, contentType string, body io.Reader) (*http.Response, error) {
	return s.Do(context.TODO(), Method("DELETE"), URL(url), Header("Content-Type", contentType), Body(body))
}

// Head send post request
func Head(url string) (resp *http.Response, err error) {
	return s.Do(context.Background(), Method("HEAD"), URL(url))
}

// PostForm send post request,  content-type = application/x-www-form-urlencoded
func PostForm(url string, data url.Values) (*http.Response, error) {
	return s.Do(context.TODO(), MethodPost, URL(url), Header("Content-Type", "application/x-www-form-urlencoded"),
		Body(strings.NewReader(data.Encode())),
	)
}
