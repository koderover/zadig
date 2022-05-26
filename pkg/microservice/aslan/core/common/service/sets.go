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

package service

type hookUniqueID struct {
	owner, namespace, repo, source, name string
}

type hookItem struct {
	hookUniqueID
	codeHostID int
}

type Empty struct{}

type HookSet map[hookUniqueID]hookItem

// NewHookSet creates a HookSet from a list of values.
func NewHookSet(items ...hookItem) HookSet {
	ss := HookSet{}
	ss.Insert(items...)
	return ss
}

// Insert adds items to the set.
func (s HookSet) Insert(items ...hookItem) HookSet {
	for _, item := range items {
		s[item.hookUniqueID] = item
	}
	return s
}

// Has returns true if and only if item is contained in the set.
func (s HookSet) Has(item hookItem) bool {
	_, contained := s[item.hookUniqueID]
	return contained
}

// Difference returns a set of objects that are not in s2
// For example:
// s1 = {a1, a2, a3}
// s2 = {a1, a2, a4, a5}
// s1.Difference(s2) = {a3}
// s2.Difference(s1) = {a4, a5}
func (s HookSet) Difference(s2 HookSet) HookSet {
	result := NewHookSet()
	for _, v := range s {
		if !s2.Has(v) {
			result.Insert(v)
		}
	}
	return result
}

// Len returns the size of the set.
func (s HookSet) Len() int {
	return len(s)
}
