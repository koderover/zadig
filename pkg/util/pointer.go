/*
Copyright 2022 The KodeRover Authors.

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

func GetBoolPointer(data bool) *bool {
	return &data
}

func GetStrPointer(data string) *string {
	if len(data) == 0 {
		return nil
	}

	return &data
}

func GetIntPointer(data int) *int {
	return &data
}

func GetInt32Pointer(data int32) *int32 {
	return &data
}

func GetInt64Pointer(data int64) *int64 {
	return &data
}

func GetBoolFromPointer(source *bool) bool {
	if source == nil {
		return false
	}
	return *source
}

func GetStringFromPointer(source *string) string {
	if source == nil {
		return ""
	}
	return *source
}

func GetIntFromPointer(source *int) int {
	if source == nil {
		return 0
	}
	return *source
}

func GetInt32FromPointer(source *int32) int32 {
	if source == nil {
		return 0
	}
	return *source
}

func GetInt64FromPointer(source *int64) int64 {
	if source == nil {
		return 0
	}
	return *source
}
