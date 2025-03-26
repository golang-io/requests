package requests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

// LogS supply default handle Stat, print to stdout.
func LogS(ctx context.Context, stat *Stat) {
	if stat.Response.URL == "" {
		_, _ = fmt.Printf("%s\n", stat)
		return
	}
	if b, err := json.Marshal(stat.Request.Body); err != nil {
		log.Printf(`%s # body=%v, resp="%v", err=%v`, stat.Print(), stat.Request.Body, stat.Response.Body, err)
	} else {
		log.Printf(`%s # body=%s, resp="%v"`, stat.Print(), b, stat.Response.Body)
	}
}
