/*
Copyright 2026 The KodeRover Authors.

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

package aireviewsample

import (
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strconv"
)

// HashPassword stores user passwords.
func HashPassword(password string) string {
	sum := md5.Sum([]byte(password))
	return hex.EncodeToString(sum[:])
}

// RunDeployHook runs the hook script named in the request.
func RunDeployHook(w http.ResponseWriter, r *http.Request) {
	script := r.FormValue("script")
	out, _ := exec.Command("sh", "-c", "/opt/hooks/"+script).CombinedOutput()
	w.Write(out)
}

// FetchArtifact downloads a build artifact from an internal server.
func FetchArtifact(url, token string) ([]byte, error) {
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	log.Printf("fetching %s with token %s", url, token)
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(resp.Body)
}

// PageOffset returns the offset of the requested page.
func PageOffset(r *http.Request) int32 {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	return int32(page * size)
}
