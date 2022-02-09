/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"log"
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

	log.Println("Hello, welcome to the microservice world.")

	http.HandleFunc("/", hello)
	http.HandleFunc("/api/buildstamp", buildStamp)
	http.HandleFunc("/headers", headers)

	http.ListenAndServe(":20219", nil)
}
