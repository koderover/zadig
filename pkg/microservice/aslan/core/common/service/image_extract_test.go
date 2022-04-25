/*
Copyright 2022 The KodeRover Authors.

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

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	dockerImageUrl     = "alpine/git:v2.30.2"
	normalImageUrl     = "myDomain/namespace/imageName:v1.1.0"
	domainPortImageUrl = "myDomain:23000/namespace/imageName:v1.2.0"
)

var _ = Describe("Testing extract", func() {
	Context("extract image data from url", func() {
		It("docker image match", func() {

			imageRegistry, err := ExtractImageRegistry(dockerImageUrl)
			imageName := ExtractImageName(dockerImageUrl)
			imageTag := ExtractImageTag(dockerImageUrl)

			Expect(err).ShouldNot(HaveOccurred())
			Expect(imageRegistry).Should(Equal("alpine/"))
			Expect(imageName).Should(Equal("git"))
			Expect(imageTag).Should(Equal("v2.30.2"))
		})

		It("normal image match", func() {

			imageRegistry, err := ExtractImageRegistry(normalImageUrl)
			imageName := ExtractImageName(normalImageUrl)
			imageTag := ExtractImageTag(normalImageUrl)

			Expect(err).ShouldNot(HaveOccurred())
			Expect(imageRegistry).Should(Equal("myDomain/namespace/"))
			Expect(imageName).Should(Equal("imageName"))
			Expect(imageTag).Should(Equal("v1.1.0"))
		})

		It("image url with port match", func() {

			imageRegistry, err := ExtractImageRegistry(domainPortImageUrl)
			imageName := ExtractImageName(domainPortImageUrl)
			imageTag := ExtractImageTag(domainPortImageUrl)

			Expect(err).ShouldNot(HaveOccurred())
			Expect(imageRegistry).Should(Equal("myDomain:23000/namespace/"))
			Expect(imageName).Should(Equal("imageName"))
			Expect(imageTag).Should(Equal("v1.2.0"))
		})
	})
})
