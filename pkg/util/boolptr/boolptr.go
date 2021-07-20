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

package boolptr

// IsTrue returns true if and only if the bool pointer is non-nil and set to true.
func IsTrue(b *bool) bool {
	return b != nil && *b
}

// IsFalse returns true if and only if the bool pointer is non-nil and set to false.
func IsFalse(b *bool) bool {
	return b != nil && !*b
}

// True returns a *bool whose underlying value is true.
func True() *bool {
	t := true
	return &t
}

// False returns a *bool whose underlying value is false.
func False() *bool {
	t := false
	return &t
}

// Equal returns true if and only if both values are set and equal.
func Equal(a, b *bool) bool {
	if a == nil || b == nil {
		return false
	}

	return *a == *b
}

// NilOrEqual returns true if both values are set and equal or both are nil.
func NilOrEqual(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	} else if a == nil || b == nil {
		return false
	} else {
		return *a == *b
	}
}
