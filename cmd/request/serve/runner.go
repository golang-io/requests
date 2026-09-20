package serve

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/golang-io/requests"
)

// Run executes the HTTP request with the provided flags and URL.
func Run(f *Flags, rawURL string) error {
	opts, err := BuildOptions(f, rawURL)
	if err != nil {
		return err
	}

	sess := requests.New()
	resp, err := sess.Do(context.Background(), opts...)
	if err != nil {
		if !f.Silent {
			fmt.Fprintf(os.Stderr, "request: %v\n", err)
		}
		return err
	}
	defer resp.Body.Close()

	// Dump headers to file (-D)
	if f.DumpHeader != "" {
		if herr := DumpHeaders(f.DumpHeader, resp); herr != nil && !f.Silent {
			fmt.Fprintf(os.Stderr, "request: dump-header: %v\n", herr)
		}
	}

	// Determine output writer
	out := io.Writer(os.Stdout)
	if f.Output != "" {
		file, err := os.Create(f.Output) // #nosec G304
		if err != nil {
			return fmt.Errorf("cannot open output file: %w", err)
		}
		defer file.Close()
		out = file
	}

	// Verbose: print request info to stderr
	if f.Verbose {
		PrintVerboseRequest(resp.Request, os.Stderr)
		fmt.Fprintf(os.Stderr, "< %s\r\n", resp.Status)
		for k, vv := range resp.Header {
			for _, v := range vv {
				fmt.Fprintf(os.Stderr, "< %s: %s\r\n", k, v)
			}
		}
		fmt.Fprintf(os.Stderr, "<\r\n")
	}

	// -i: include response headers in output
	if f.Include {
		PrintResponseHeaders(resp, out)
	}

	// -I (HEAD): only print headers, no body
	if f.Head {
		if !f.Include && !f.Verbose {
			PrintResponseHeaders(resp, out)
		}
		return nil
	}

	_, err = io.Copy(out, resp.Body)
	return err
}
