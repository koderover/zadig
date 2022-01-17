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
	"fmt"
	"time"
)

// Age returns a string representing the time elapsed since unixTime in the form "1d", "2h", "3m", or "4s".
func Age(unixTime int64) string {
	duration := time.Now().Unix() - unixTime

	if duration >= 0 && duration < 60 {
		return fmt.Sprintf("%ds", duration)
	}

	if duration >= 60 && duration < 60*60 {
		return fmt.Sprintf("%dm", duration/60)
	}

	if duration >= 60*60 && duration < 60*60*24 {
		return fmt.Sprintf("%dh", duration/(60*60))
	}

	if duration >= 60*60*24 {
		return fmt.Sprintf("%dd", duration/(60*60*24))
	}
	return "0s"
}
