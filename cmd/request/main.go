package main

import (
	"fmt"
	"os"

	"github.com/golang-io/requests/cmd/request/serve"
	"github.com/spf13/cobra"
)

func main() {
	f := &serve.Flags{}

	rootCmd := &cobra.Command{
		Use:   "request [options] <url>",
		Short: "A curl-compatible HTTP client powered by golang-io/requests",
		Long: `request - transfer a URL, curl-compatible interface

Examples:
  request https://httpbin.org/get
  request -X POST -H "Content-Type: application/json" -d '{"key":"val"}' https://httpbin.org/post
  request -u user:pass https://httpbin.org/basic-auth/user/pass
  request -F "file=@/path/to/file" https://httpbin.org/post
  request -o /tmp/out.html https://example.com
  request -k -v https://self-signed.badssl.com/`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return serve.Run(f, args[0])
		},
	}

	pf := rootCmd.Flags()

	// ----- Request method & body -----
	pf.StringVarP(&f.Method, "request", "X", "", "Specify request method (GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS)")
	pf.StringArrayVarP(&f.Headers, "header", "H", nil, "Pass custom header(s) to server (repeatable)")
	pf.StringVarP(&f.Data, "data", "d", "", "HTTP POST data (prefix @ to read from file)")
	pf.StringVar(&f.DataRaw, "data-raw", "", "HTTP POST data, no @file processing")
	pf.StringVar(&f.DataAscii, "data-ascii", "", "HTTP POST data (ASCII, same as -d)")
	pf.StringVar(&f.DataBinary, "data-binary", "", "HTTP POST data in binary mode")
	pf.StringVar(&f.DataURLEnc, "data-urlencode", "", "HTTP POST data URL-encoded")
	pf.StringVar(&f.JSONData, "json", "", "HTTP POST JSON and set Content-Type/Accept to application/json")
	pf.StringArrayVarP(&f.FormItems, "form", "F", nil, "Multipart form field (name=value or name=@file)")
	pf.StringArrayVar(&f.FormStrings, "form-string", nil, "Multipart form string field (never treated as file)")
	pf.StringVarP(&f.UploadFile, "upload-file", "T", "", "Transfer local file via HTTP PUT")

	// ----- Auth -----
	pf.StringVarP(&f.User, "user", "u", "", "Server user and password (user:password)")

	// ----- Output -----
	pf.StringVarP(&f.Output, "output", "o", "", "Write output to FILE instead of stdout")
	pf.BoolVarP(&f.Silent, "silent", "s", false, "Silent mode")
	pf.BoolVarP(&f.Verbose, "verbose", "v", false, "Make the operation more talkative")
	pf.BoolVarP(&f.Include, "include", "i", false, "Include response headers in output")
	pf.BoolVarP(&f.Head, "head", "I", false, "HTTP HEAD method")
	pf.StringVarP(&f.DumpHeader, "dump-header", "D", "", "Write received headers to FILE")

	// ----- Connection -----
	pf.StringVarP(&f.Proxy, "proxy", "x", "", "Use specified proxy (protocol://host:port)")
	pf.BoolVarP(&f.Insecure, "insecure", "k", false, "Allow insecure connections (skip TLS verify)")
	pf.IntVar(&f.Timeout, "max-time", 0, "Maximum time (seconds) for the whole operation")
	pf.IntVar(&f.MaxRedirs, "max-redirs", 10, "Maximum number of redirects (-1 = unlimited)")
	pf.BoolVarP(&f.Location, "location", "L", false, "Follow redirects")

	// ----- Cookie -----
	pf.StringVarP(&f.Cookie, "cookie", "b", "", "Send cookies from string or FILE")
	pf.StringVarP(&f.CookieJar, "cookie-jar", "c", "", "Write cookies to FILE (placeholder)")

	// ----- TLS -----
	pf.StringVar(&f.Cert, "cert", "", "Client certificate file")
	pf.StringVar(&f.Key, "key", "", "Private key file")

	// ----- Misc -----
	pf.BoolVar(&f.Compressed, "compressed", false, "Request compressed response")
	pf.StringVarP(&f.UserAgent, "user-agent", "A", "", "Send User-Agent string to server")
	pf.StringVarP(&f.Referer, "referer", "e", "", "Referrer URL")
	pf.StringVarP(&f.RangeSpec, "range", "r", "", "Retrieve byte range (e.g. 0-1023)")
	pf.StringVarP(&f.ContinueAt, "continue-at", "C", "", "Resumed transfer offset ('-' for auto)")

	if err := rootCmd.Execute(); err != nil {
		if !f.Silent {
			fmt.Fprintf(os.Stderr, "request: %v\n", err)
		}
		os.Exit(1)
	}
}
