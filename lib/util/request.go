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

package util

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"os"
)

func SendRequest(url, method string, header http.Header, body []byte) ([]byte, error) {
	request, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	request.Header = header

	var ret *http.Response
	client := &http.Client{}
	ret, err = client.Do(request)
	if err != nil {
		return nil, err
	}

	defer func() { _ = ret.Body.Close() }()

	var resp []byte
	resp, err = ioutil.ReadAll(ret.Body)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func GetRequestBody(body interface{}) io.Reader {
	if body == nil {
		return nil
	}
	dataStr, ok := body.(string)
	if ok {
		return bytes.NewReader([]byte(dataStr))
	}
	dataBytes, ok := body.([]byte)
	if ok {
		return bytes.NewReader(dataBytes)
	}
	rawData, err := json.Marshal(body)
	if err != nil {
		return nil
	}
	return bytes.NewReader(rawData)
}

func DownloadFile(url string, filepath string) error {
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}
