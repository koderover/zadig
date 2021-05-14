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

package mongo_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

type testParams struct {
	uri, expectedDBName, expectedURI string
}

var _ = Describe("Testing options", func() {

	DescribeTable("Testing ExtractDatabaseName",
		func(p testParams) {
			db, newURI, err := mongotool.ExtractDatabaseName(p.uri)

			Expect(err).ShouldNot(HaveOccurred())
			Expect(db).To(Equal(p.expectedDBName))
			Expect(newURI).To(Equal(p.expectedURI))
		},
		Entry("uri with database name", testParams{
			uri:            "mongodb://0.0.0.0:27017/loutest",
			expectedDBName: "loutest",
			expectedURI:    "mongodb://0.0.0.0:27017/",
		}),
		Entry("uri with database name and options", testParams{
			uri:            "mongodb://0.0.0.0:27017/loutest?a=b",
			expectedDBName: "loutest",
			expectedURI:    "mongodb://0.0.0.0:27017/?a=b",
		}),
		Entry("uri with database name and credentials", testParams{
			uri:            "mongodb://user:pass@0.0.0.0:27017/loutest",
			expectedDBName: "loutest",
			expectedURI:    "mongodb://user:pass@0.0.0.0:27017/",
		}),
		Entry("uri without database name", testParams{
			uri:            "mongodb://0.0.0.0:27017",
			expectedDBName: "",
			expectedURI:    "mongodb://0.0.0.0:27017",
		}),
		Entry("uri without database name", testParams{
			uri:            "mongodb://0.0.0.0:27017,0.0.0.0:27017,0.0.0.0:27017/",
			expectedDBName: "",
			expectedURI:    "mongodb://0.0.0.0:27017,0.0.0.0:27017,0.0.0.0:27017/",
		}),
		Entry("uri with parameter, which have a string same as db name", testParams{
			uri:            "mongodb://koderover:password@0.0.0.0:30001,0.0.0.0:30001,0.0.0.0:30001,0.0.0.0:30001,0.0.0.0:30001/koderover?authSource=koderover",
			expectedDBName: "koderover",
			expectedURI:    "mongodb://koderover:password@0.0.0.0:30001,0.0.0.0:30001,0.0.0.0:30001,0.0.0.0:30001,0.0.0.0:30001/?authSource=koderover",
		}),
		Entry("uri with parameter, which have multiple strings same as db name", testParams{
			uri:            "mongodb://koderover:password@koderover.com:30001,10.108.27.171:30001,10.108.27.172:30001,10.108.27.170:30001,10.108.27.174:30001/koderover?koderover=koderover",
			expectedDBName: "koderover",
			expectedURI:    "mongodb://koderover:password@koderover.com:30001,10.108.27.171:30001,10.108.27.172:30001,10.108.27.170:30001,10.108.27.174:30001/?koderover=koderover",
		}),
		Entry("uri with parameter, which have multiple strings same as db name", testParams{
			uri:            "mongodb://koderover:password@koderover.com:30001,10.108.27.171:30001,10.108.27.172:30001,10.108.27.170:30001,10.108.27.174:30001/koderover?koderover=koderover?xd=xd",
			expectedDBName: "koderover",
			expectedURI:    "mongodb://koderover:password@koderover.com:30001,10.108.27.171:30001,10.108.27.172:30001,10.108.27.170:30001,10.108.27.174:30001/?koderover=koderover?xd=xd",
		}),
	)
})
