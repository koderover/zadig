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

package terminalio

type Recorder interface {
	RecordInput(data string)
	RecordOutput(data string)
	RecordResize(cols, rows uint16)
}

type Sanitizer interface {
	Mask(data string) string
}

func ProcessOutput(raw string, recorder Recorder, sanitizer Sanitizer) string {
	sanitized := raw
	if sanitizer != nil {
		sanitized = sanitizer.Mask(raw)
	}
	if recorder != nil {
		recorder.RecordOutput(sanitized)
	}
	return sanitized
}
