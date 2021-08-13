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

package upgradepath

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Testing dag", func() {
	var called0To1, called1To2, called0To2, called1To0 bool

	AfterEach(func() {
		reset()

		called0To1 = false
		called1To2 = false
		called0To2 = false
		called1To0 = false
	})

	Context("upgradeWithBestPath finds a long path", func() {

		BeforeEach(func() {
			AddHandler(0, 1, func() error {
				called0To1 = true
				return nil
			})
			AddHandler(1, 2, func() error {
				called1To2 = true
				return nil
			})
		})

		It("should upgrade version by version", func() {
			err := upgradeWithBestPath(0, 2)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(called0To1).To(BeTrue())
			Expect(called1To2).To(BeTrue())
		})
	})

	Context("upgradeWithBestPath finds a short path", func() {

		BeforeEach(func() {
			AddHandler(0, 1, func() error {
				called0To1 = true
				return nil
			})
			AddHandler(1, 2, func() error {
				called1To2 = true
				return nil
			})
			AddHandler(0, 2, func() error {
				called0To2 = true
				return nil
			})
		})

		It("should upgrade cross version", func() {
			err := upgradeWithBestPath(0, 2)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(called0To1).To(BeFalse())
			Expect(called1To2).To(BeFalse())
			Expect(called0To2).To(BeTrue())
		})
	})

	Context("upgradeWithBestPath rollbacks to old version", func() {

		BeforeEach(func() {
			AddHandler(1, 0, func() error {
				called1To0 = true
				return nil
			})
		})

		It("should rollback to right version", func() {
			err := upgradeWithBestPath(1, 0)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(called1To0).To(BeTrue())
		})
	})
})
