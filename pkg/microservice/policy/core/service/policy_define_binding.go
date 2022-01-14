package service

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
)

type PolicyDefineBinding struct {
	Name string `json:"name"`
	UID  string `json:"uid"`
	Role string `json:"role"`
}

func CreatePolicyDefineBindings(ns string, pbs []*PolicyDefineBinding, logger *zap.SugaredLogger) error {
	var objs []*models.PolicyDefineBinding
	for _, pb := range pbs {
		obj, err := createPolicyDefineBindingObject(ns, pb, logger)
		if err != nil {
			return err
		}

		objs = append(objs, obj)
	}

	return mongodb.NewPolicyBindingColl().BulkCreate(objs)
}

func createPolicyDefineBindingObject(ns string, pb *PolicyDefineBinding, logger *zap.SugaredLogger) (*models.PolicyDefineBinding, error) {
	nsRole := ns

	role, found, err := mongodb.NewPolicyDefineColl().Get(nsRole, pb.Role)
	if err != nil {
		logger.Errorf("Failed to get role %s in namespace %s, err: %s", pb.Role, nsRole, err)
		return nil, err
	} else if !found {
		logger.Errorf("Role %s is not found in namespace %s", pb.Role, nsRole)
		return nil, fmt.Errorf("role %s not found", pb.Role)
	}

	return &models.PolicyDefineBinding{
		Name:      pb.Name,
		Namespace: ns,
		Subjects:  []*models.Subject{{Kind: models.UserKind, UID: pb.UID}},
		RoleRef: &models.RoleRef{
			Name:      role.Name,
			Namespace: role.Namespace,
		},
	}, nil
}
