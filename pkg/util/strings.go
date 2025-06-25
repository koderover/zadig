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
	"regexp"
	"strings"
	"unicode"

	ref "github.com/containers/image/docker/reference"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/mozillazg/go-pinyin"
)

func GetJiraKeys(title string) (keys []string) {

	re := regexp.MustCompile("[a-zA-Z0-9]+-[0-9]+")
	keys = re.FindAllString(title, -1)
	return
}

func ReplaceWrapLine(script string) string {
	return strings.Replace(strings.Replace(
		script,
		"\r\n",
		"\n",
		-1,
	), "\r", "\n", -1)
}

// Test case reference https://github.com/containers/image/blob/main/docker/reference/reference_test.go
func ExtractImageName(image string) string {
	imageNameStr := ""

	reference, err := ref.Parse(image)
	if err != nil {
		return imageNameStr
	}
	if named, ok := reference.(ref.Named); ok {
		imageNameStr = named.Name()
	}

	return imageNameStr
}

func GetImageNameFromContainerInfo(imageName, containerName string) string {
	if imageName == "" {
		return containerName
	}
	return imageName
}

func RemoveExtraSpaces(input string) string {
	// Remove spaces before and after strings
	trimmed := strings.TrimSpace(input)

	// Replace multiple consecutive spaces in the middle of a string with a single space
	regex := regexp.MustCompile(`\s+`)
	normalized := regex.ReplaceAllString(trimmed, " ")

	return normalized
}

func ContainsChinese(str string) bool {
	for _, r := range str {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func GetPinyinFromChinese(han string) (string, string) {
	firstLetter := ""
	fullLetter := ""
	a := pinyin.NewArgs()
	pinyinArr := pinyin.Pinyin(han, a)
	for _, pinyin := range pinyinArr {
		for _, l := range pinyin {
			fullLetter += l
			firstLetter += string(l[0])
		}
	}
	return fullLetter, firstLetter
}

func GetKeyAndInitials(name string) (string, string) {
	firstLetter := ""
	fullLetter := ""
	a := pinyin.NewArgs()
	for _, r := range name {
		if unicode.Is(unicode.Han, r) {
			pinyinArr := pinyin.Pinyin(string(r), a)
			for _, pinyin := range pinyinArr {
				for _, l := range pinyin {
					fullLetter += l
					firstLetter += string(l[0])
				}
			}
		} else {
			fullLetter += string(r)
			firstLetter += string(r)
		}
	}
	return fullLetter, firstLetter
}

func GetEnvSleepCronName(projectName, envName string, isEnable bool) string {
	suffix := "sleep"
	if !isEnable {
		suffix = "awake"
	}
	return fmt.Sprintf("%s-%s-%s-%s", envName, projectName, setting.EnvSleepCronjob, suffix)
}

func GetReleasePlanCronName(id, releasePlanName string, index int64) string {
	return fmt.Sprintf("%s-%s-%d-%s", id, releasePlanName, index, setting.ReleasePlanCronjob)
}
