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

package workflow

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Testing utils", func() {

	Context("validateHookNames", func() {
		It("should be passed for valid names", func() {
			err := validateHookNames([]string{"a"})
			Expect(err).ShouldNot(HaveOccurred())
		})
		It("should raise error for empty name", func() {
			err := validateHookNames([]string{"a", ""})
			Expect(err).Should(HaveOccurred())
		})
		It("should raise error for invalid characters", func() {
			err := validateHookNames([]string{"a", "*"})
			Expect(err).Should(HaveOccurred())
		})
		It("should raise error for duplicated names", func() {
			err := validateHookNames([]string{"a", "a"})
			Expect(err).Should(HaveOccurred())
		})
	})
})
