/*
 * Copyright 2026 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import (
	"encoding/json"
)

func hasReleasePlanSnapshotChanges(beforeSnapshot, afterSnapshot interface{}) bool {
	beforeComparable := normalizeReleasePlanSnapshotComparableValue("", beforeSnapshot)
	afterComparable := normalizeReleasePlanSnapshotComparableValue("", afterSnapshot)
	return !releasePlanSnapshotValuesEqual(beforeComparable, afterComparable)
}

func releasePlanSnapshotValuesEqual(left, right interface{}) bool {
	switch leftValue := left.(type) {
	case map[string]interface{}:
		rightValue, ok := right.(map[string]interface{})
		if !ok {
			return isEmptyReleasePlanSnapshotValue(left) && isEmptyReleasePlanSnapshotValue(right)
		}
		return releasePlanSnapshotMapsEqual(leftValue, rightValue)
	case []interface{}:
		rightValue, ok := right.([]interface{})
		if !ok {
			return isEmptyReleasePlanSnapshotValue(left) && isEmptyReleasePlanSnapshotValue(right)
		}
		return releasePlanSnapshotListsEqual(leftValue, rightValue)
	default:
		switch right.(type) {
		case map[string]interface{}, []interface{}:
			return isEmptyReleasePlanSnapshotValue(left) && isEmptyReleasePlanSnapshotValue(right)
		}
	}

	if isEmptyReleasePlanSnapshotScalarValue(left) && isEmptyReleasePlanSnapshotScalarValue(right) {
		return true
	}
	leftNumber, leftIsNumber := releasePlanSnapshotNumber(left)
	rightNumber, rightIsNumber := releasePlanSnapshotNumber(right)
	if leftIsNumber || rightIsNumber {
		return leftIsNumber && rightIsNumber && leftNumber == rightNumber
	}
	return left == right
}

func releasePlanSnapshotMapsEqual(left, right map[string]interface{}) bool {
	for key, leftValue := range left {
		rightValue, exists := right[key]
		if !exists {
			if !isEmptyReleasePlanSnapshotValue(leftValue) {
				return false
			}
			continue
		}
		if !releasePlanSnapshotValuesEqual(leftValue, rightValue) {
			return false
		}
	}
	for key, rightValue := range right {
		if _, exists := left[key]; !exists && !isEmptyReleasePlanSnapshotValue(rightValue) {
			return false
		}
	}
	return true
}

func releasePlanSnapshotListsEqual(left, right []interface{}) bool {
	leftIdx, rightIdx := 0, 0
	for {
		for leftIdx < len(left) && isEmptyReleasePlanSnapshotValue(left[leftIdx]) {
			leftIdx++
		}
		for rightIdx < len(right) && isEmptyReleasePlanSnapshotValue(right[rightIdx]) {
			rightIdx++
		}
		if leftIdx == len(left) || rightIdx == len(right) {
			break
		}
		if !releasePlanSnapshotValuesEqual(left[leftIdx], right[rightIdx]) {
			return false
		}
		leftIdx++
		rightIdx++
	}
	for leftIdx < len(left) {
		if !isEmptyReleasePlanSnapshotValue(left[leftIdx]) {
			return false
		}
		leftIdx++
	}
	for rightIdx < len(right) {
		if !isEmptyReleasePlanSnapshotValue(right[rightIdx]) {
			return false
		}
		rightIdx++
	}
	return true
}

func isEmptyReleasePlanSnapshotValue(value interface{}) bool {
	switch typedValue := value.(type) {
	case map[string]interface{}:
		for _, item := range typedValue {
			if !isEmptyReleasePlanSnapshotValue(item) {
				return false
			}
		}
		return true
	case []interface{}:
		for _, item := range typedValue {
			if !isEmptyReleasePlanSnapshotValue(item) {
				return false
			}
		}
		return true
	default:
		return isEmptyReleasePlanSnapshotScalarValue(value)
	}
}

func isEmptyReleasePlanSnapshotScalarValue(value interface{}) bool {
	switch typedValue := value.(type) {
	case nil:
		return true
	case string:
		return typedValue == ""
	case bool:
		return !typedValue
	default:
		number, ok := releasePlanSnapshotNumber(value)
		return ok && number == 0
	}
}

func releasePlanSnapshotNumber(value interface{}) (float64, bool) {
	switch typedValue := value.(type) {
	case int:
		return float64(typedValue), true
	case int8:
		return float64(typedValue), true
	case int16:
		return float64(typedValue), true
	case int32:
		return float64(typedValue), true
	case int64:
		return float64(typedValue), true
	case uint:
		return float64(typedValue), true
	case uint8:
		return float64(typedValue), true
	case uint16:
		return float64(typedValue), true
	case uint32:
		return float64(typedValue), true
	case uint64:
		return float64(typedValue), true
	case float32:
		return float64(typedValue), true
	case float64:
		return typedValue, true
	case json.Number:
		number, err := typedValue.Float64()
		return number, err == nil
	default:
		return 0, false
	}
}
