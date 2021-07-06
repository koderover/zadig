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
	"math/rand"
	"time"

	"github.com/google/uuid"
)

func UUID() string {
	return uuid.New().String()
}

const str = "abcdefghijklmnopqrstuvwxyz"
const numStr = "0123456789abcdefghijklmnopqrstuvwxyz"

func GetRandomNumString(length int) string {
	res := make([]byte, length)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := range res {
		res[i] = numStr[r.Intn(len(numStr))]
	}
	return string(res)
}

func GetRandomString(length int) string {
	res := make([]byte, length)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := range res {
		res[i] = numStr[r.Intn(len(str))]
	}
	return string(res)
}
