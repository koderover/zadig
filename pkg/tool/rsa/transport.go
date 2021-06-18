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

package rsa

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

// SignTransport ...
type SignTransport struct {
	t http.RoundTripper
}

// NewSignTransport ...
func NewSignTransport(t http.RoundTripper) http.RoundTripper {
	return &SignTransport{t: t}
}

// RoundTrip ...
func (t *SignTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	sh := crypto.SHA1.New()
	if req.Body != nil {
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, req.Body)
		if err != nil {
			return nil, err
		}
		sh.Write(buf.Bytes())
		req.Body = ioutil.NopCloser(buf)
	}
	timestamp := strconv.FormatInt(time.Now().UnixNano(), 10)
	req.Header.Add("TimeStamp", timestamp)
	reqURI := req.URL.RequestURI()
	sh.Write([]byte(timestamp))
	sh.Write([]byte(reqURI))
	hash := sh.Sum(nil)
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA1, hash)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", hex.EncodeToString(sig))
	return t.t.RoundTrip(req)
}

// DefaultClient ...
var DefaultClient = &http.Client{
	Transport: NewSignTransport(http.DefaultTransport),
}
