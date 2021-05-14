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

package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

var UserAgent = "KodeRover Client"

// TODO: lou: implement a complete http client.
type Client struct {
	*http.Client
}

func (r Client) doRequestWith(
	ctx context.Context, method, url1 string,
	bodyType string, body io.Reader, bodyLength int) (resp *http.Response, err error) {

	req, err := http.NewRequest(method, url1, body)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", bodyType)
	req.ContentLength = int64(bodyLength)
	return r.do(ctx, req)
}

func (r Client) DoRequestWithJson(
	ctx context.Context, method, url1 string, data interface{}) (resp *http.Response, err error) {

	msg, err := json.Marshal(data)
	if err != nil {
		return
	}
	return r.doRequestWith(
		ctx, method, url1, "application/json", bytes.NewReader(msg), len(msg))
}

func (r Client) do(ctx context.Context, req *http.Request) (resp *http.Response, err error) {

	if ctx == nil {
		ctx = context.Background()
	}

	if _, ok := req.Header["User-Agent"]; !ok {
		req.Header.Set("User-Agent", UserAgent)
	}

	transport := r.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	// avoid cancel() is called before Do(req), but isn't accurate
	select {
	case <-ctx.Done():
		err = ctx.Err()
		return
	default:
	}

	if tr, ok := getRequestCanceler(transport); ok { // support CancelRequest
		reqC := make(chan bool, 1)
		go func() {
			resp, err = r.Client.Do(req)
			reqC <- true
		}()
		select {
		case <-reqC:
		case <-ctx.Done():
			tr.CancelRequest(req)
			<-reqC
			err = ctx.Err()
		}
	} else {
		resp, err = r.Client.Do(req)
	}
	return
}

// --------------------------------------------------------------------

type ErrorInfo struct {
	Err   string `json:"error,omitempty"`
	Key   string `json:"key,omitempty"`
	Errno int    `json:"errno,omitempty"`
	Code  int    `json:"code"`
}

func (r *ErrorInfo) ErrorDetail() string {

	msg, _ := json.Marshal(r)
	return string(msg)
}

func (r *ErrorInfo) Error() string {

	return r.Err
}

func (r *ErrorInfo) RpcError() (code, errno int, key, err string) {

	return r.Code, r.Errno, r.Key, r.Err
}

func (r *ErrorInfo) HttpCode() int {

	return r.Code
}

// --------------------------------------------------------------------

func parseError(e *ErrorInfo, r io.Reader) {

	body, err1 := ioutil.ReadAll(r)
	if err1 != nil {
		e.Err = err1.Error()
		return
	}

	var ret struct {
		Err   string `json:"error"`
		Key   string `json:"key"`
		Errno int    `json:"errno"`
	}
	if json.Unmarshal(body, &ret) == nil && ret.Err != "" {
		e.Err, e.Key, e.Errno = ret.Err, ret.Key, ret.Errno
		return
	}
	e.Err = string(body)
}

func ResponseError(resp *http.Response) (err error) {

	e := &ErrorInfo{
		Code: resp.StatusCode,
	}
	if resp.StatusCode > 299 {
		if resp.ContentLength != 0 {
			ct, ok := resp.Header["Content-Type"]
			if ok && strings.HasPrefix(ct[0], "application/json") {
				parseError(e, resp.Body)
			}
		}
	}
	return e
}

func callRet(ret interface{}, resp *http.Response) (err error) {

	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode/100 == 2 {
		if ret != nil && resp.ContentLength != 0 {
			err = json.NewDecoder(resp.Body).Decode(ret)
			if err != nil {
				return
			}
		}
		if resp.StatusCode == 200 {
			return nil
		}
	}
	return ResponseError(resp)
}

func (r Client) CallWithJson(
	ctx context.Context, ret interface{}, method, url1 string, param interface{}) (err error) {

	resp, err := r.DoRequestWithJson(ctx, method, url1, param)
	if err != nil {
		return err
	}
	return callRet(ret, resp)
}

func (r Client) Call(
	ctx context.Context, ret interface{}, method, url1 string) (err error) {

	resp, err := r.doRequestWith(ctx, method, url1, "application/x-www-form-urlencoded", nil, 0)
	if err != nil {
		return err
	}
	return callRet(ret, resp)
}

// ---------------------------------------------------------------------------

type requestCanceler interface {
	CancelRequest(req *http.Request)
}

type nestedObjectGetter interface {
	NestedObject() interface{}
}

func getRequestCanceler(tp http.RoundTripper) (rc requestCanceler, ok bool) {

	if rc, ok = tp.(requestCanceler); ok {
		return
	}

	p := interface{}(tp)
	for {
		getter, ok1 := p.(nestedObjectGetter)
		if !ok1 {
			return
		}
		p = getter.NestedObject()
		if rc, ok = p.(requestCanceler); ok {
			return
		}
	}
}
