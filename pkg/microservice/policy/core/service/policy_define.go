package service

import (
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
)

func CreatePolicyDefine(policyDefine *models.PolicyDefine) error {
	return mongodb.NewPolicyDefineColl().Create(policyDefine)

}
