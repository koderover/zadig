package service

import (
	"fmt"
	"testing"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
)

func TestUpdateWorkloads(t *testing.T) {
	exist := []models.Workload{
		models.Workload{
			EnvName:     "env1",
			Name:        "service1",
			ProductName: "product1",
		},
		models.Workload{
			EnvName:     "env1",
			Name:        "service2",
			ProductName: "product1",
		},
	}
	diff := make(map[string]*ServiceWorkloads, 0)
	diff["service1"] = &ServiceWorkloads{
		EnvName:     "env1",
		Name:        "service1",
		ProductName: "product1",
		Operation:   "delete",
	}
	diff["service3"] = &ServiceWorkloads{
		EnvName:     "env1",
		Name:        "service3",
		ProductName: "product1",
		Operation:   "add",
	}

	result := updateWorkloads(exist, diff, "env1")
	fmt.Println(result)
}
