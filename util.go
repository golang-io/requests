package requests

import (
	"bytes"
	"context"
	"fmt"
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
	if err := r.Close(); err != nil {
		return &buf, err
	}
	return &buf, nil
}

// CopyBody reads all of b to memory and then returns two equivalent
// ReadClosers yielding the same bytes.
//
// It returns an error if the initial slurp of all bytes fails. It does not attempt
// to make the returned ReadClosers have identical error-matching behavior.
func CopyBody(b io.ReadCloser) (*bytes.Buffer, io.ReadCloser, error) {
	var buf bytes.Buffer
	if b == nil || b == http.NoBody {
		// No copying needed. Preserve the magic sentinel meaning of NoBody.
		return &buf, http.NoBody, nil
	}
	if _, err := buf.ReadFrom(b); err != nil {
		return &buf, b, err
	}
	if err := b.Close(); err != nil {
		return &buf, b, err
	}
	return &buf, io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

// Log print
func Log(format string, v ...any) {
	_, _ = fmt.Printf(format+"\n", v...)
}

// LogS supply default handle Stat, print to stdout.
func LogS(_ context.Context, stat *Stat) {
	Log("%s\n", stat)
}

// StreamS supply default handle Stream, print raw msg in stream to stdout.
func StreamS(i int64, raw []byte) error {
	_, err := fmt.Printf("i=%d, raw=%s\n", i, raw)
	return err
}
