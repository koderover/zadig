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
	"log"

	"github.com/RyanCarrier/dijkstra"
)

type handler func() error

type upgradePath struct {
	from, to int
}

var dag = dijkstra.NewGraph()
var handlerMap = make(map[upgradePath]handler)

func AddHandler(from, to int, fn handler) {
	if v, err := dag.GetVertex(from); err != nil || v.ID != from {
		dag.AddVertex(from)
	}
	if v, err := dag.GetVertex(to); err != nil || v.ID != to {
		dag.AddVertex(to)
	}
	if err := dag.AddArc(from, to, 1); err != nil {
		log.Fatal(err)
	}

	handlerMap[upgradePath{from: from, to: to}] = fn
}

func UpgradeWithBestPath(from, to string) error {
	return upgradeWithBestPath(versionMap.From(from), versionMap.To(to))
}

func upgradeWithBestPath(from, to int) error {
	if from == to {
		return nil
	}
	best, err := dag.Shortest(from, to)
	if err != nil {
		// no upgrade path is found
		return err
	}

	path := best.Path
	var s, t int
	for len(path) > 1 {
		s = path[0]
		t = path[1]
		path = path[1:]

		fn := handlerMap[upgradePath{from: s, to: t}]
		if err = fn(); err != nil {
			return err
		}
	}

	return nil
}

func reset() {
	dag = dijkstra.NewGraph()
}
