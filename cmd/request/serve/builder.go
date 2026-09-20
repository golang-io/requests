package serve

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-io/requests"
)

// BuildOptions converts CLI flags into a requests.Option slice.
func BuildOptions(f *Flags, rawURL string) ([]requests.Option, error) {
	var opts []requests.Option

	opts = append(opts, requests.URL(rawURL))
	opts = append(opts, requests.Method(ResolveMethod(f)))

	// ---- Timeout ----
	if f.Timeout > 0 {
		opts = append(opts, requests.Timeout(time.Duration(f.Timeout)*time.Second))
	}

	// ---- TLS ----
	if f.Insecure {
		opts = append(opts, requests.Verify(false))
	}
	if f.Cert != "" && f.Key != "" {
		opts = append(opts, requests.CertKey(f.Cert, f.Key))
	}

	// ---- Proxy ----
	if f.Proxy != "" {
		opts = append(opts, requests.Proxy(f.Proxy))
	}

	// ---- Custom headers ----
	for _, h := range f.Headers {
		k, v, ok := strings.Cut(h, ":")
		if !ok {
			return nil, fmt.Errorf("invalid header %q (expected 'Name: Value')", h)
		}
		opts = append(opts, requests.Header(strings.TrimSpace(k), strings.TrimSpace(v)))
	}

	// ---- Misc headers ----
	if f.UserAgent != "" {
		opts = append(opts, requests.Header("User-Agent", f.UserAgent))
	}
	if f.Referer != "" {
		opts = append(opts, requests.Header("Referer", f.Referer))
	}
	if f.RangeSpec != "" {
		opts = append(opts, requests.Header("Range", "bytes="+f.RangeSpec))
	}
	if f.ContinueAt != "" && f.ContinueAt != "-" {
		opts = append(opts, requests.Header("Range", "bytes="+f.ContinueAt+"-"))
	}
	if f.Compressed {
		opts = append(opts, requests.Header("Accept-Encoding", "gzip, deflate, br"))
	}

	// ---- Auth ----
	if f.User != "" {
		user, pass, _ := strings.Cut(f.User, ":")
		opts = append(opts, requests.BasicAuth(user, pass))
	}

	// ---- Cookies ----
	if f.Cookie != "" {
		if IsFile(f.Cookie) {
			if cookieStr, err := ParseCookieFile(f.Cookie); err == nil {
				opts = append(opts, requests.Header("Cookie", cookieStr))
			}
		} else {
			opts = append(opts, requests.Header("Cookie", f.Cookie))
		}
	}

	// ---- Body ----
	bodyOpts, err := buildBodyOptions(f)
	if err != nil {
		return nil, err
	}
	opts = append(opts, bodyOpts...)

	return opts, nil
}

// ResolveMethod determines the HTTP method from flags.
func ResolveMethod(f *Flags) string {
	if f.Method != "" {
		return strings.ToUpper(f.Method)
	}
	if f.Head {
		return http.MethodHead
	}
	if f.UploadFile != "" {
		return http.MethodPut
	}
	hasBody := f.Data != "" || f.DataRaw != "" || f.DataAscii != "" ||
		f.DataBinary != "" || f.DataURLEnc != "" || f.JSONData != "" ||
		len(f.FormItems) > 0 || len(f.FormStrings) > 0
	if hasBody {
		return http.MethodPost
	}
	return http.MethodGet
}

// buildBodyOptions builds body-related options from data flags.
func buildBodyOptions(f *Flags) ([]requests.Option, error) {
	// --json
	if f.JSONData != "" {
		return []requests.Option{
			requests.Header("Content-Type", "application/json"),
			requests.Header("Accept", "application/json"),
			requests.Body(strings.NewReader(f.JSONData)),
		}, nil
	}

	// -F / --form → multipart
	if len(f.FormItems) > 0 || len(f.FormStrings) > 0 {
		reader, ct, err := buildMultipart(f.FormItems, f.FormStrings)
		if err != nil {
			return nil, err
		}
		return []requests.Option{
			requests.Header("Content-Type", ct),
			requests.Body(reader),
		}, nil
	}

	// -T: file upload via PUT
	if f.UploadFile != "" {
		file, err := os.Open(f.UploadFile) // #nosec G304
		if err != nil {
			return nil, fmt.Errorf("cannot open upload file: %w", err)
		}
		return []requests.Option{requests.Body(file)}, nil
	}

	// Raw body data flags
	rawBody := ""
	isURLEncoded := false
	switch {
	case f.DataBinary != "":
		rawBody = f.DataBinary
	case f.DataRaw != "":
		rawBody = f.DataRaw
	case f.DataAscii != "":
		rawBody = f.DataAscii
	case f.Data != "":
		rawBody = f.Data
		isURLEncoded = true
	case f.DataURLEnc != "":
		rawBody = url.QueryEscape(f.DataURLEnc)
		isURLEncoded = true
	}

	if rawBody == "" {
		return nil, nil
	}

	var bodyOpts []requests.Option
	if isURLEncoded {
		bodyOpts = append(bodyOpts, requests.Header("Content-Type", "application/x-www-form-urlencoded"))
	}

	// curl: prefix '@' means read from file
	if strings.HasPrefix(rawBody, "@") {
		path := rawBody[1:]
		file, err := os.Open(path) // #nosec G304
		if err != nil {
			return nil, fmt.Errorf("cannot open data file %q: %w", path, err)
		}
		bodyOpts = append(bodyOpts, requests.Body(file))
		return bodyOpts, nil
	}

	bodyOpts = append(bodyOpts, requests.Body(strings.NewReader(rawBody)))
	return bodyOpts, nil
}

// buildMultipart constructs a multipart/form-data body from form items.
// Form item syntax: "name=value" or "name=@/path/to/file[;type=mime][;filename=override]"
func buildMultipart(formItems, formStrings []string) (io.Reader, string, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	go func() {
		var writeErr error

		for _, item := range formStrings {
			k, v, _ := strings.Cut(item, "=")
			if writeErr = mw.WriteField(k, v); writeErr != nil {
				break
			}
		}

		if writeErr == nil {
			for _, item := range formItems {
				k, v, _ := strings.Cut(item, "=")
				if strings.HasPrefix(v, "@") {
					writeErr = writeFormFile(mw, k, v[1:])
				} else {
					writeErr = mw.WriteField(k, v)
				}
				if writeErr != nil {
					break
				}
			}
		}

		mw.Close()
		pw.CloseWithError(writeErr)
	}()

	return pr, mw.FormDataContentType(), nil
}

// writeFormFile writes a file field into the multipart writer.
// spec format: "/path/to/file[;type=mime][;filename=override]"
func writeFormFile(mw *multipart.Writer, fieldName, spec string) error {
	filePath, params := parseFormFileSpec(spec)
	fname := params["filename"]
	if fname == "" {
		fname = filepath.Base(filePath)
	}

	var (
		fw  io.Writer
		err error
	)
	if mimeType := params["type"]; mimeType != "" {
		h := make(map[string][]string)
		h["Content-Disposition"] = []string{
			fmt.Sprintf(`form-data; name=%q; filename=%q`, fieldName, fname),
		}
		h["Content-Type"] = []string{mimeType}
		fw, err = mw.CreatePart(h)
	} else {
		fw, err = mw.CreateFormFile(fieldName, fname)
	}
	if err != nil {
		return err
	}

	f, err := os.Open(filePath) // #nosec G304
	if err != nil {
		return fmt.Errorf("cannot open form file %q: %w", filePath, err)
	}
	defer f.Close()

	_, err = io.Copy(fw, f)
	return err
}

// parseFormFileSpec splits "/path[;key=val;...]" into (path, params).
func parseFormFileSpec(spec string) (path string, params map[string]string) {
	params = make(map[string]string)
	parts := strings.Split(spec, ";")
	path = parts[0]
	for _, p := range parts[1:] {
		k, v, ok := strings.Cut(p, "=")
		if ok {
			params[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return
}
