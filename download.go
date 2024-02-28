package requests

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// WriteCounter WriteCounter
type WriteCounter struct {
	Max   uint64
	Total uint64
}

// Write xx
func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Total += uint64(n)
	wc.PrintProgress()
	return n, nil
}

// PrintProgress prints the progress of a file write
func (wc *WriteCounter) PrintProgress() {
	// Clear the line by using a character return to go back to the start and remove
	// the remaining characters by filling it with spaces
	fmt.Printf("\r%s", strings.Repeat(" ", 50))
	fmt.Printf("\rDownloading... %d complete[%.2f%%]", wc.Total, float64(wc.Total*100)/float64(wc.Max))
}

// DownloadFile download file
func DownloadFile(url string, progress bool, filepath ...string) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	size, err := strconv.Atoi(resp.Header.Get("Content-Length"))
	if err != nil {
		return err
	}

	var filename string
	if len(filepath) != 0 {
		filename = filepath[0]
	} else {
		disposition := resp.Header.Get("Content-Disposition")
		filename = strings.Trim(strings.SplitN(disposition, "filename=", 2)[1], `"`)
	}

	var reader io.Reader
	if progress {
		// Create our bytes counter and pass it to be used alongside our writer
		counter := &WriteCounter{Max: uint64(size)}
		reader = io.TeeReader(resp.Body, counter)
	} else {
		reader = resp.Body
	}

	// Create the file with .tmp extension, so that we won't overwrite a file until it's downloaded fully
	tmpfile, err := os.CreateTemp(".", "download-")
	if err != nil {
		return err
	}

	if _, err = io.Copy(tmpfile, reader); err != nil {
		return err
	}

	if err = tmpfile.Close(); err != nil {
		return err
	}
	// The progress use the same line so print a new line once it's finished downloading
	fmt.Println()
	// Rename the tmp file back to the original file
	return os.Rename(tmpfile.Name(), filename)

}
