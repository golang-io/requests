package main

import (
	"bytes"
	"github.com/golang-io/requests"
	"log"
)

func main() {
	resp, err := requests.Get("https://httpbin.org/get")
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		log.Fatalln(err)
	}
	log.Println(buf.String())
}
