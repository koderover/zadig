package service

import (
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
)

func CreatePolicyDefine(policyDefine *models.PolicyDefine) error {
	return mongodb.NewPolicyDefineColl().Create(policyDefine)
}

func ListPolicyDefine(projectName string) ([]*models.PolicyDefine, error) {
	return mongodb.NewPolicyDefineColl().List(projectName)
}

func DeletePolicyDefine(id string) error {
	return mongodb.NewPolicyDefineColl().Delete(id)
}
