package requests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// makeBody converts various input types to an io.Reader suitable for HTTP request bodies.
// Supported types:
// - nil: returns nil (no body)
// - []byte: returns a bytes.Reader
// - string: returns a strings.Reader
// - *bytes.Buffer, bytes.Buffer: returns as io.Reader
// - io.Reader, io.ReadSeeker, *bytes.Reader, *strings.Reader: returns as is
// - url.Values: returns encoded form values as strings.Reader
// - func() (io.ReadCloser, error): calls the function and returns the result
// - any other type: marshals to JSON and returns as bytes.Reader
func makeBody(body any) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}

	switch v := body.(type) {
	case []byte:
		return bytes.NewReader(v), nil
	case string:
		return strings.NewReader(v), nil
	case *bytes.Buffer, bytes.Buffer:
		return body.(io.Reader), nil
	case io.Reader, io.ReadSeeker, *bytes.Reader, *strings.Reader:
		return body.(io.Reader), nil
	case url.Values:
		return strings.NewReader(v.Encode()), nil
	case func() (io.ReadCloser, error):
		return v()
	default:
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(b), nil
	}
}

// NewRequestWithContext creates a new HTTP request with the given context and options.
// It handles:
// - Converting the request body to an appropriate io.Reader
// - Setting the request method and URL
// - Appending path segments
// - Setting query parameters
// - Setting headers and cookies
//
// Returns the constructed http.Request and any error encountered during creation.
func NewRequestWithContext(ctx context.Context, options Options) (*http.Request, error) {
	body, err := makeBody(options.body)
	if err != nil {
		return nil, err
	}

	r, err := http.NewRequestWithContext(ctx, options.Method, options.URL, body)
	if err != nil {
		return nil, err
	}

	for _, p := range options.Path {
		r.URL.Path += p
	}

	r.URL.RawQuery = options.RawQuery.Encode()

	r.Header = options.Header
	for _, cookie := range options.Cookies {
		r.AddCookie(&cookie)
	}
	return r, nil
}
