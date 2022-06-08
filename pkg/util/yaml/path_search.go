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
	"math"
	"strings"

	"k8s.io/utils/strings/slices"
)

const (
	separator = "."
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

type patternDataMap map[string]map[string]*patternData

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

func (pdm patternDataMap) addPatternMap(name, rPath, aPath string) *patternData {
	pData := &patternData{
		name:         name,
		relativePath: rPath,
		absolutePath: aPath,
	}
	if _, ok := pdm[name]; !ok {
		pdm[name] = make(map[string]*patternData)
	}
	pdm[name][aPath] = pData
	return pData
}

func (pdm patternDataMap) getAPathByName(name string) []string {
	ret := make([]string, 0)
	if pData, ok := pdm[name]; ok {
		for _, data := range pData {
			ret = append(ret, data.absolutePath)
		}
	}
	return ret
}

type patternData struct {
	name         string
	relativePath string
	absolutePath string
	usedPrefix   string
}

type pathResult struct {
	name         string
	absolutePath string
}

type pathResultSet struct {
	resultSet []*pathResult
	prefix    string
}

// paths share the same longest prefix
type relativePathSet struct {
	paths []string
}

type relativePathChecker struct {
	relativePaths map[string]*relativePathSet
}

func strSliceContainsAnother(source, target []string) bool {
	for _, str := range target {
		if !slices.Contains(source, str) {
			return false
		}
	}
	return true
}

// check if the input path set contains relative paths
func (checker *relativePathChecker) checkMatchRelativePath(paths []string) bool {
	for _, path := range paths {
		if relativePath, ok := checker.relativePaths[path]; ok {
			if strSliceContainsAnother(paths, relativePath.paths) {
				return true
			} else {
				return false
			}
		}
	}
	return true
}

func newRelativeChecker(rPaths [][]string) *relativePathChecker {
	ret := &relativePathChecker{relativePaths: map[string]*relativePathSet{}}
	for _, pathSet := range rPaths {
		relativeData := &relativePathSet{
			paths: pathSet,
		}
		for _, path := range pathSet {
			ret.relativePaths[path] = relativeData
		}
	}
	return ret
}

// find the longest common prefix from string list
func longestCommonPrefix(strList []string) string {
	n := len(strList)
	if n == 0 {
		return ""
	} else if n == 1 {
		return strList[0]
	}
	minStrLen := math.MaxInt32
	for i := 0; i < n; i++ {
		if minStrLen > len(strList[i]) {
			minStrLen = len(strList[i])
		}
	}
	prefix := strList[0][0:minStrLen]
	var allFound bool
	for {
		allFound = true
		for i := 1; i < n; i++ {
			if strings.Index(strList[i], prefix) != 0 {
				prefix = prefix[0 : len(prefix)-1]
				allFound = false
				break
			}
		}
		if allFound || len(prefix) == 0 {
			break
		}
	}
	return prefix
}

func findCombinations(arr [][]string, checker *relativePathChecker) [][]string {
	ret := make([][]string, 0)
	n := len(arr)
	indices := make([]int, n, n)
	for {
		singleSlice := make([]string, 0)
		for i := 0; i < n; i++ {
			singleSlice = append(singleSlice, arr[i][indices[i]])
		}
		if len(singleSlice) > 0 {
			if checker.checkMatchRelativePath(singleSlice) {
				ret = append(ret, singleSlice)
			}
		}

		// find the rightmost array that has more
		// elements left after the current element
		// in that array
		next := n - 1
		for {
			if next >= 0 && (indices[next]+1 >= len(arr[next])) {
				next--
			} else {
				break
			}
		}
		if next < 0 {
			break
		}
		// if found move to next element in that
		// array
		indices[next] = indices[next] + 1

		// for all arrays to the right of this
		// array current index again points to
		// first element
		for i := next + 1; i < n; i++ {
			indices[i] = 0
		}
	}

	return ret
}

func toPathResult(prefix string, path []string, aPatternData map[string]*patternData) *pathResultSet {
	ret := &pathResultSet{
		prefix: prefix,
	}
	for _, aPath := range path {
		pData, _ := aPatternData[aPath]
		if len(pData.usedPrefix) <= len(prefix) {
			pData.usedPrefix = prefix
		}
		ret.resultSet = append(ret.resultSet, &pathResult{
			name:         pData.name,
			absolutePath: aPath,
		})
	}
	return ret
}

func processResults(resultSets []*pathResultSet, aPatternData map[string]*patternData, pattern map[string]string) []map[string]string {
	ret := make([]map[string]string, 0)
	for _, singleSet := range resultSets {
		retMap := make(map[string]string)

		useLongestPrefix := true
		for _, result := range singleSet.resultSet {
			pData := aPatternData[result.absolutePath]
			if len(pData.usedPrefix) != len(singleSet.prefix) {
				useLongestPrefix = false
				break
			}
		}
		if !useLongestPrefix {
			continue
		}

		for _, result := range singleSet.resultSet {
			retMap[result.name] = result.absolutePath
		}
		if len(pattern) != len(retMap) {
			continue
		}
		ret = append(ret, retMap)
	}
	return ret
}

func (isr *pathSearcher) checkRelativePath(k string) {
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
				},
			},
		}
		isr.resultSet[prefix] = fs
		return
	}
}

func (isr *pathSearcher) getRelativePath() [][]string {
	ret := make([][]string, 0)
	for _, fSet := range isr.resultSet {
		// multi keys share the same prefix
		if len(fSet.foundSet) <= 1 {
			continue
		}
		set := make([]string, 0)
		for _, searchInfo := range fSet.foundSet {
			set = append(set, searchInfo.absolutePath)
		}
		ret = append(ret, set)
	}
	return ret
}

func search(flatMap map[string]interface{}, pattern map[string]string) ([]map[string]string, error) {

	patternDataMap := patternDataMap{}
	allPatternData := make(map[string]*patternData) // absolutePath => patternData
	relativeSearcher := &pathSearcher{
		pattern:   pattern,
		resultSet: make(map[string]*pathSearchRuntime),
	}

	for absolutePath := range flatMap {
		for patternName, relativePath := range pattern {
			if relativePath != absolutePath && !strings.HasSuffix(absolutePath, fmt.Sprintf(".%s", relativePath)) {
				continue
			}
			pData := patternDataMap.addPatternMap(patternName, relativePath, absolutePath)
			allPatternData[absolutePath] = pData
		}
		relativeSearcher.checkRelativePath(absolutePath)
	}

	relativePaths := relativeSearcher.getRelativePath()
	checker := newRelativeChecker(relativePaths)

	allKeys := make([][]string, 0)
	for name, _ := range patternDataMap {
		pathList := patternDataMap.getAPathByName(name)
		if len(pathList) == 0 {
			return nil, nil
		}
		allKeys = append(allKeys, pathList)
	}

	allCombinations := findCombinations(allKeys, checker)

	results := make([]*pathResultSet, 0)

	for _, cb := range allCombinations {
		lp := longestCommonPrefix(cb)
		result := toPathResult(lp, cb, allPatternData)
		results = append(results, result)
	}

	return processResults(results, allPatternData, pattern), nil
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
func SearchByPattern(flatMap map[string]interface{}, patterns []map[string]string) ([]map[string]string, error) {
	err := preFilterSearchPaths(patterns)
	if err != nil {
		return nil, err
	}

	ret := make([]map[string]string, 0)
	for _, singlePattern := range patterns {
		pRet, err := search(flatMap, singlePattern)
		if err != nil {
			return nil, err
		}
		ret = append(ret, pRet...)
	}
	return ret, nil
}
