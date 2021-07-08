package main

import (
	"fmt"
	"net/http"
)

var BuildStamp = "No Build Stamp Provided"

func hello(w http.ResponseWriter, req *http.Request) {

	fmt.Fprintf(w, "hello, my name is Go~~\n")
}

func headers(w http.ResponseWriter, req *http.Request) {

	for name, headers := range req.Header {
		for _, h := range headers {
			fmt.Fprintf(w, "%v: %v\n", name, h)
		}
	}
}

func buildStamp(w http.ResponseWriter, req *http.Request) {

	fmt.Fprintf(w, "%s", BuildStamp)
}

func main() {

	http.HandleFunc("/", hello)
	http.HandleFunc("/api/buildstamp", buildStamp)
	http.HandleFunc("/headers", headers)

	http.ListenAndServe(":20219", nil)
}
