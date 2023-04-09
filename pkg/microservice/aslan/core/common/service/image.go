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

//// parse image url to map: repo=>xxx/xx/xx image=>xx tag=>xxx
//func resolveImageUrl(imageUrl string) map[string]string {
//	subMatchAll := imageParseRegex.FindStringSubmatch(imageUrl)
//	result := make(map[string]string)
//	exNames := imageParseRegex.SubexpNames()
//	for i, matchedStr := range subMatchAll {
//		if i != 0 && matchedStr != "" && matchedStr != ":" {
//			result[exNames[i]] = matchedStr
//		}
//	}
//	return result
//}
//
//// AssignImageData assign image url data into match data
//// matchData: image=>absolute-path repo=>absolute-path tag=>absolute-path
//// return: absolute-image-path=>image-value  absolute-repo-path=>repo-value absolute-tag-path=>tag-value
//func AssignImageData(imageUrl string, matchData map[string]string) (map[string]interface{}, error) {
//	ret := make(map[string]interface{})
//	// total image url assigned into one single value
//	if len(matchData) == 1 {
//		for _, v := range matchData {
//			ret[v] = imageUrl
//		}
//		return ret, nil
//	}
//
//	resolvedImageUrl := resolveImageUrl(imageUrl)
//
//	// image url assigned into repo/image+tag
//	if len(matchData) == 3 {
//		ret[matchData[setting.PathSearchComponentRepo]] = strings.TrimSuffix(resolvedImageUrl[setting.PathSearchComponentRepo], "/")
//		ret[matchData[setting.PathSearchComponentImage]] = resolvedImageUrl[setting.PathSearchComponentImage]
//		ret[matchData[setting.PathSearchComponentTag]] = resolvedImageUrl[setting.PathSearchComponentTag]
//		return ret, nil
//	}
//
//	if len(matchData) == 2 {
//		// image url assigned into repo/image + tag
//		if tagPath, ok := matchData[setting.PathSearchComponentTag]; ok {
//			ret[tagPath] = resolvedImageUrl[setting.PathSearchComponentTag]
//			for k, imagePath := range matchData {
//				if k == setting.PathSearchComponentTag {
//					continue
//				}
//				ret[imagePath] = fmt.Sprintf("%s%s", resolvedImageUrl[setting.PathSearchComponentRepo], resolvedImageUrl[setting.PathSearchComponentImage])
//				break
//			}
//			return ret, nil
//		}
//		// image url assigned into repo + image(tag)
//		ret[matchData[setting.PathSearchComponentRepo]] = strings.TrimSuffix(resolvedImageUrl[setting.PathSearchComponentRepo], "/")
//		ret[matchData[setting.PathSearchComponentImage]] = fmt.Sprintf("%s:%s", resolvedImageUrl[setting.PathSearchComponentImage], resolvedImageUrl[setting.PathSearchComponentTag])
//		return ret, nil
//	}
//
//	return nil, fmt.Errorf("match data illegal, expect length: 1-3, actual length: %d", len(matchData))
//}
//
//// ReplaceImage replace image defines in yaml by new version
//func ReplaceImage(sourceYaml string, imageValuesMap ...map[string]interface{}) (string, error) {
//	bytes := [][]byte{[]byte(sourceYaml)}
//	for _, imageValues := range imageValuesMap {
//		nestedMap, err := converter.Expand(imageValues)
//		if err != nil {
//			return "", err
//		}
//		bs, err := yaml.Marshal(nestedMap)
//		if err != nil {
//			return "", err
//		}
//		bytes = append(bytes, bs)
//	}
//
//	mergedBs, err := yamlutil.Merge(bytes)
//	if err != nil {
//		return "", err
//	}
//	return string(mergedBs), nil
//}
