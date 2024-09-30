package requests

import (
	"bytes"
	"io"
	"net/http"
)

// ParseBody parse body from `Request.Body`.
func ParseBody(r io.ReadCloser) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	if r == nil || r == http.NoBody {
		// No copying needed. Preserve the magic sentinel meaning of NoBody.
		return &buf, nil
	}
	if _, err := buf.ReadFrom(r); err != nil {
		return &buf, err
	}
	return &buf, r.Close()
}

// CopyBody reads all of b to memory and then returns two equivalent
// ReadClosers yielding the same bytes.
//
// It returns an error if the initial slurp of all bytes fails. It does not attempt
// to make the returned ReadClosers have identical error-matching behavior.
func CopyBody(b io.ReadCloser) (*bytes.Buffer, io.ReadCloser, error) {
	buf, err := ParseBody(b)
	if err != nil {
		return nil, nil, err
	}
	return buf, io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}
