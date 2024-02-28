package requests

import (
	"io"
	"net/url"
)

var sess = New()

// Get send get request
func Get(url string) (*Response, error) {
	return sess.Get(url)
}

// Post send post request
func Post(url string, contentType string, body io.Reader) (*Response, error) {
	return sess.Post(url, contentType, body)
}

// PUT send put request
func PUT(url, contentType string, body io.Reader) (*Response, error) {
	return sess.Put(url, contentType, body)
}

// Delete send delete request
func Delete(url, contentType string, body io.Reader) (*Response, error) {
	return sess.Delete(url, contentType, body)
}

// Head send post request
func Head(url string) (resp *Response, err error) {
	return sess.Head(url)
}

// PostForm send post request,  content-type = application/x-www-form-urlencoded
func PostForm(url string, data url.Values) (*Response, error) {
	return sess.PostForm(url, data)
}

// Wget download a file from remote.
func Wget(url, name string) (int, error) {
	resp, err := sess.Get(url)
	if err != nil {
		return 0, err
	}
	return resp.Download(name)
}
