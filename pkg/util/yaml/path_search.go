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

package yaml

import (
	"errors"
	"fmt"
	"strings"
)

func getKey(prefix, k string) string {
	if prefix == "" {
		return k
	}
	return fmt.Sprintf("%s.%s", prefix, k)
}

type singlePathSearchInfo struct {
	pathName     string
	relativePath string
	absolutePath string
	value        interface{}
}

type pathSearchRuntime struct {
	pattern  map[string]string
	prefix   string
	foundSet map[string]*singlePathSearchInfo // path => singlePathSearchInfo
}

func (searchRuntime *pathSearchRuntime) checkAllFinish() bool {
	for _, singlePath := range searchRuntime.pattern {
		if singlePath == "" {
			continue
		}
		if _, ok := searchRuntime.foundSet[singlePath]; !ok {
			return false
		}
	}
	return true
}

type pathSearcher struct {
	pattern   map[string]string
	resultSet map[string]*pathSearchRuntime //prefix => search result
}

func (isr *pathSearcher) handleKV(k string, v interface{}) {
	expectPaths, preMatch := isr.pattern, false
	for _, ep := range expectPaths {
		if strings.HasSuffix(k, ep) {
			preMatch = true
			break
		}
	}
	if !preMatch {
		return
	}

	//优先匹配当前已经match了一部分path的情形
	for prefix, fs := range isr.resultSet {
		if fs.checkAllFinish() {
			continue
		}
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		match := false
		for pathName, ep := range expectPaths {
			//已经找到了相关的实际path pass
			if _, ok := fs.foundSet[ep]; ok {
				continue
			} else {
				if !strings.HasSuffix(k, ep) {
					continue
				}
				if getKey(prefix, ep) != k {
					break
				}
				fs.foundSet[ep] = &singlePathSearchInfo{
					pathName:     pathName,
					relativePath: ep,
					absolutePath: k,
					value:        v,
				}
				match = true
			}
		}
		if match {
			return
		}
	}

	for pathName, ep := range expectPaths {
		if !strings.HasSuffix(k, ep) {
			continue
		}
		prefix := strings.TrimSuffix(k, ep)
		prefix = strings.Trim(prefix, ".")
		fs := &pathSearchRuntime{
			pattern: expectPaths,
			prefix:  prefix,
			foundSet: map[string]*singlePathSearchInfo{
				ep: &singlePathSearchInfo{
					pathName:     pathName,
					relativePath: ep,
					absolutePath: k,
					value:        v,
				},
			},
		}
		isr.resultSet[prefix] = fs
		return
	}
}

// check if contains empty patterns
func preFilterSearchPaths(configs []map[string]string) error {
	for _, singleConfig := range configs {
		for k, v := range singleConfig {
			if k == "" || v == "" {
				return errors.New("empty pattern name or pattern path is not supported")
			}
		}
	}
	return nil
}

// build pathSearcher object for every pattern
func searchPaths(patterns []map[string]string, sourceMap map[string]interface{}) ([]*pathSearcher, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	err := preFilterSearchPaths(patterns)
	if err != nil {
		return nil, err
	}

	rtSlice := make([]*pathSearcher, 0)
	for _, cfg := range patterns {
		rt := &pathSearcher{
			pattern:   cfg,
			resultSet: make(map[string]*pathSearchRuntime),
		}
		rtSlice = append(rtSlice, rt)
	}
	for k, v := range sourceMap {
		for _, rt := range rtSlice {
			rt.handleKV(k, v)
		}
	}
	return rtSlice, nil
}

// merge results and filter duplicates paths
func mergeResults(isrList []*pathSearcher) []map[string]string {
	ret := make([]map[string]string, 0)
	usedPath := make(map[string]int)
	for _, isr := range isrList {
		for _, fSet := range isr.resultSet {
			retSet := make(map[string]string)
			if !fSet.checkAllFinish() {
				continue
			}
			duplicateCheckPass := true
			for _, searchInfo := range fSet.foundSet {
				if _, ok := usedPath[searchInfo.absolutePath]; ok {
					duplicateCheckPass = false
					break
				}
				retSet[searchInfo.pathName] = searchInfo.absolutePath
				usedPath[searchInfo.absolutePath] = 1
			}
			if duplicateCheckPass {
				ret = append(ret, retSet)
			}
		}
	}
	return ret
}

// SearchByPattern find all matched absolute paths from yaml by the pattern appointed
// input pattern: []map{name => relative-path}
// output: app paths found which match the relative-path rule, []{map[name]=>absolute-path}
// eg: sourceYaml:
// ---------- yaml begin
//env: dev
//svc1:
//  image:
//    repository: go-sample-site
//    tag: "0.2.1"
//svc2:
//  image:
//    repository: go-sample-site-2
//    tag: "0.2.2"
//imagePullSecrets:
//  - name: default-secret
// ----------- yaml end
// pattern: []{"image": "repository", "tag": "tag"}
// output: []{{"image": "svc1.image.repository", "tag": "svc1.image.tag"}, {"image": "svc2.image.repository", "tag": "svc2.image.tag"}}
func SearchByPattern(flatMap map[string]interface{}, pattern []map[string]string) ([]map[string]string, error) {
	foundResult, err := searchPaths(pattern, flatMap)
	if err != nil {
		return nil, err
	}
	ret := mergeResults(foundResult)
	return ret, nil
}
