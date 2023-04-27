/*
 * Copyright 2023 The KodeRover Authors.
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

package validator

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IsValidImageName", func() {
	It("returns false for an empty string", func() {
		Expect(IsValidImageName("")).To(BeFalse())
	})

	It("returns false for an invalid character", func() {
		Expect(IsValidImageName("ubuntu：123")).To(BeFalse())
	})

	It("returns true for a valid image name without tag", func() {
		Expect(IsValidImageName("ubuntu")).To(BeTrue())
	})

	It("returns false for an invalid image name with invalid characters", func() {
		Expect(IsValidImageName("{ubuntu")).To(BeFalse())
	})

	It("returns false for an invalid image name with tag containing colon only", func() {
		Expect(IsValidImageName("ubuntu:")).To(BeFalse())
	})

	It("returns true for a valid image name with tag and registry", func() {
		Expect(IsValidImageName("docker.io/ubuntu:123")).To(BeTrue())
	})

	It("returns true for a valid image name with tag 'latest'", func() {
		Expect(IsValidImageName("ubuntu:latest")).To(BeTrue())
	})

	It("returns false for an invalid image name with tag containing hyphen", func() {
		Expect(IsValidImageName("ubuntu{:latest")).To(BeFalse())
	})

	It("returns true for a valid image name with tag containing hyphen", func() {
		Expect(IsValidImageName("ubuntu:la-test")).To(BeTrue())
	})

	It("returns false for an invalid image name with trailing slash in tag", func() {
		Expect(IsValidImageName("ubuntu:la-test/")).To(BeFalse())
	})

	It("returns true for a valid image name with tag and registry containing dots and hyphens", func() {
		Expect(IsValidImageName("kr.example.com/test/1/nginx:20060102150405")).To(BeTrue())
	})
})

func TestIsImageName(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Validator Suite")
}
