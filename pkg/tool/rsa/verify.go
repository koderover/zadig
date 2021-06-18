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
	"crypto/rsa"
	"encoding/hex"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

// VerifyHTTPRequest ...
func VerifyHTTPRequest(req *http.Request) (err error) {
	timestamp := req.Header.Get("TimeStamp")
	if timestamp == "" {
		return errors.New("timestamp is nil")
	}
	s := req.Header.Get("Authorization")
	if s == "" {
		return errors.New("auth is nil")
	}
	sig, err := hex.DecodeString(s)
	if err != nil {
		return errors.New("decode err:" + s)
	}
	t, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return errors.New("parse err:" + err.Error())
	}
	if time.Now().UnixNano()-t > time.Hour.Nanoseconds() {
		return errors.New("time Limited Error")
	}
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, req.Body)
	if err != nil {
		return
	}
	sh := crypto.SHA1.New()
	sh.Write(buf.Bytes())
	sh.Write([]byte(timestamp))
	sh.Write([]byte(req.URL.RequestURI()))
	hash := sh.Sum(nil)
	req.Body = ioutil.NopCloser(buf)
	return rsa.VerifyPKCS1v15(pub, crypto.SHA1, hash, sig)
}
