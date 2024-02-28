package requests

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

// Upload HttpUpload
func Upload(url, field, file string) (*http.Response, error) {
	buf := new(bytes.Buffer)
	writer := multipart.NewWriter(buf)
	formFile, err := writer.CreateFormFile(field, file)
	if err != nil {
		return nil, err
	}

	srcFile, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer srcFile.Close()
	_, err = io.Copy(formFile, srcFile)
	if err != nil {
		return nil, err
	}

	contentType := writer.FormDataContentType()
	writer.Close()
	resp, err := http.Post(url, contentType, buf)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return resp, err
}
