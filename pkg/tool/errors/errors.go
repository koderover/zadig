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

package errors

import (
	"fmt"
	"regexp"
)

// IHTTPError ...
type IHTTPError interface {
	Code() int
	Error() string
	Desc() string
	Extra() map[string]interface{}
}

// HTTPError ...
type HTTPError struct {
	code  int
	err   string
	desc  string
	extra map[string]interface{}
}

// NewHTTPError ...
func NewHTTPError(code int, errStr string, args ...string) *HTTPError {

	var desc string
	if len(args) > 0 {
		desc = args[0]
	}

	return &HTTPError{
		code: code,
		err:  errStr,
		desc: desc,
	}
}

// Code ...
func (e *HTTPError) Code() int {
	return e.code
}

// Error ...
func (e *HTTPError) Error() string {
	return e.err
}

// Desc ...
func (e *HTTPError) Desc() string {
	return e.desc
}

// Extra ...
func (e *HTTPError) Extra() map[string]interface{} {
	extra := map[string]interface{}{}

	for k, v := range e.extra {
		extra[k] = v
	}

	return extra
}

// AddDesc ...
func (e *HTTPError) AddDesc(desc string) *HTTPError {
	// set default description error
	e.desc = desc

	if matched, _ := regexp.MatchString(".*E11000 duplicate.*", desc); matched {
		e.desc = "mongo duplicate key error"
	}

	return e
}

// AddErr ...
func (e *HTTPError) AddErr(err error) *HTTPError {
	e.desc = err.Error()
	return e
}

// NewWithDesc ...
func NewWithDesc(e error, desc string) error {
	if v, ok := e.(*HTTPError); ok {
		err := *v
		err.desc = desc
		return &err
	}

	return e
}

// NewWithExtras ...
func NewWithExtras(e error, desc string, extra map[string]interface{}) error {
	if v, ok := e.(*HTTPError); ok {
		err := *v
		err.desc = desc
		err.extra = extra
		return &err
	}

	return e
}

// ErrorMessage returns the code and message for Gins JSON helpers
func ErrorMessage(err error) (code int, message map[string]interface{}) {
	v, ok := err.(*HTTPError)
	if ok {
		code = v.Code()
		if v.Code()/1000 == 6 {
			code = ErrInvalidParam.Code()
		}
		return code, map[string]interface{}{
			"type":        "error",
			"message":     v.Error(),
			"code":        v.Code(),
			"description": v.Desc(),
			"extra":       v.Extra(),
		}
	}

	return ErrInternalError.Code(), map[string]interface{}{
		"message":     ErrInternalError.Error(),
		"code":        ErrInternalError.Code(),
		"description": err.Error(),
	}
}

// String ...
func String(err error) string {
	v, ok := err.(*HTTPError)
	if ok {
		if v.desc != "" {
			return fmt.Sprintf("%s: %s", v.Error(), v.Desc())
		}
		return v.err
	}

	return err.Error()
}
