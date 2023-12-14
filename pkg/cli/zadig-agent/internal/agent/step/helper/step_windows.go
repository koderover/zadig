//go:build windows
// +build windows

/*
Copyright 2023 The KodeRover Authors.

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

package helper

import (
	"fmt"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func handlingChineseEncoding(b []byte) ([]byte, error) {
	decodeBytes, err := simplifiedchinese.GB18030.NewDecoder().Bytes(b)
	if err != nil {
		return b, fmt.Errorf("failed to decode to GB18030, source: %s, err: %v", b, err)
	}
	return decodeBytes, nil
}
