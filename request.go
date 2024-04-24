package requests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func makeBody(body any) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}
	switch v := body.(type) {
	case []byte:
		return bytes.NewReader(v), nil
	case string:
		return strings.NewReader(v), nil
	case *bytes.Buffer:
		return bytes.NewReader(v.Bytes()), nil
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

// NewRequestWithContext request
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

	for k, v := range options.Params {
		if r.URL.RawQuery != "" {
			r.URL.RawQuery += "&"
		}
		r.URL.RawQuery += k + "=" + url.QueryEscape(fmt.Sprintf("%v", v))
	}

	r.Header = options.Header

	for _, cookie := range options.Cookies {
		r.AddCookie(&cookie)
	}

	return r, nil
}
