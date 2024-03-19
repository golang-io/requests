package requests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
)

// DumpRequest returns the given request in its HTTP/1.x wire representation.
func DumpRequest(req *http.Request) ([]byte, error) {
	return httputil.DumpRequestOut(req, true)
}

// DumpRequestIndent warp Dump
func DumpRequestIndent(req *http.Request) string {
	dump, _ := DumpRequest(req)
	var b bytes.Buffer
	for _, line := range bytes.Split(dump, []byte("\n")) {
		b.Write([]byte("> "))
		b.Write(line)
		b.WriteString("\n")
	}
	return b.String()
}

func show(prompt string, b []byte, maxTruncateBytes int) string {
	var buf bytes.Buffer
	for _, line := range bytes.Split(b, []byte("\n")) {
		buf.Write([]byte(prompt))
		buf.Write(bytes.Replace(line, []byte("%"), []byte("%%"), -1))
		buf.WriteString("\n")
	}
	str := buf.String()
	if len(str) > maxTruncateBytes {
		return fmt.Sprintf("%s...[Len=%d, Truncated[%d]]", str[:maxTruncateBytes], len(str), maxTruncateBytes)
	}
	return str
}

// drainBody reads all of b to memory and then returns two equivalent
// ReadClosers yielding the same bytes.
//
// It returns an error if the initial slurp of all bytes fails. It does not attempt
// to make the returned ReadClosers have identical error-matching behavior.
func drainBody(b io.ReadCloser) (r1, r2 io.ReadCloser, err error) {
	if b == nil || b == http.NoBody {
		// No copying needed. Preserve the magic sentinel meaning of NoBody.
		return http.NoBody, http.NoBody, nil
	}
	var buf bytes.Buffer
	if _, err = buf.ReadFrom(b); err != nil {
		return nil, b, err
	}
	if err = b.Close(); err != nil {
		return nil, b, err
	}
	return io.NopCloser(&buf), io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

func LogS(ctx context.Context, stat *Stat) {
	_, _ = fmt.Fprintf(os.Stdout, "%s\n", stat)
}

func StreamS(i int64, raw []byte) error {
	_, err := fmt.Fprintf(os.Stdout, "i=%d, raw=%s", i, raw)
	return err
}
