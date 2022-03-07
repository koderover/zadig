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

package service

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
)

type PolicyBinding struct {
	Name   string               `json:"name"`
	UID    string               `json:"uid"`
	Policy string               `json:"policy"`
	Preset bool                 `json:"preset"`
	Type   setting.ResourceType `json:"type"`
}

func CreatePolicyBindings(ns string, rbs []*PolicyBinding, logger *zap.SugaredLogger) error {
	var objs []*models.PolicyBinding
	for _, rb := range rbs {
		obj, err := createPolicyBindingObject(ns, rb, logger)
		if err != nil {
			return err
		}

		objs = append(objs, obj)
	}

	return mongodb.NewPolicyBindingColl().BulkCreate(objs)
}

func CreateOrUpdateSystemPolicyBinding(ns string, rb *PolicyBinding, logger *zap.SugaredLogger) error {
	obj, err := createPolicyBindingObject(ns, rb, logger)
	if err != nil {
		return err
	}
	return mongodb.NewPolicyBindingColl().UpdateOrCreate(obj)
}

func UpdateOrCreatePolicyBinding(ns string, rb *PolicyBinding, logger *zap.SugaredLogger) error {
	obj, err := createPolicyBindingObject(ns, rb, logger)
	if err != nil {
		return err
	}
	return mongodb.NewPolicyBindingColl().UpdateOrCreate(obj)
}

func ListPolicyBindings(ns, uid string, _ *zap.SugaredLogger) ([]*PolicyBinding, error) {
	var policyBindings []*PolicyBinding
	modelPolicyBindings, err := mongodb.NewPolicyBindingColl().ListBy(ns, uid)
	if err != nil {
		return nil, err
	}

	for _, v := range modelPolicyBindings {
		policyBindings = append(policyBindings, &PolicyBinding{
			Name:   v.Name,
			Policy: v.PolicyRef.Name,
			UID:    v.Subjects[0].UID,
			Preset: v.PolicyRef.Namespace == "",
			Type:   v.Type,
		})
	}

	return policyBindings, nil
}

func ListPolicyBindingsByPolicy(ns, policyName string, publicPolicy bool, _ *zap.SugaredLogger) ([]*PolicyBinding, error) {
	var policyBindings []*PolicyBinding

	policyNamespace := ns
	if publicPolicy {
		policyNamespace = ""
	}
	modelPolicyBindings, err := mongodb.NewPolicyBindingColl().List(&mongodb.ListPolicyOptions{PolicyName: policyName, PolicyNamespace: policyNamespace})
	if err != nil {
		return nil, err
	}

	for _, v := range modelPolicyBindings {
		policyBindings = append(policyBindings, &PolicyBinding{
			Name:   v.Name,
			Policy: v.PolicyRef.Name,
			UID:    v.Subjects[0].UID,
			Preset: v.PolicyRef.Namespace == "",
		})
	}

	return policyBindings, nil
}

func DeletePolicyBinding(name string, projectName string, _ *zap.SugaredLogger) error {
	return mongodb.NewPolicyBindingColl().Delete(name, projectName)
}

func DeletePolicyBindings(names []string, projectName string, userID string, _ *zap.SugaredLogger) error {

	if len(names) == 1 && names[0] == "*" {
		names = []string{}
	}

	return mongodb.NewPolicyBindingColl().DeleteMany(names, projectName, userID)
}

func createPolicyBindingObject(ns string, rb *PolicyBinding, logger *zap.SugaredLogger) (*models.PolicyBinding, error) {
	nsPolicy := ns
	if rb.Preset {
		nsPolicy = ""
	}
	policy, found, err := mongodb.NewPolicyColl().Get(nsPolicy, rb.Policy)
	if err != nil {
		logger.Errorf("Failed to get policy %s in namespace %s, err: %s", rb.Policy, nsPolicy, err)
		return nil, err
	} else if !found {
		logger.Errorf("Policy %s is not found in namespace %s", rb.Policy, nsPolicy)
		return nil, fmt.Errorf("policy %s not found", rb.Policy)
	}

	return &models.PolicyBinding{
		Name:      rb.Name,
		Namespace: ns,
		Subjects:  []*models.Subject{{Kind: models.UserKind, UID: rb.UID}},
		PolicyRef: &models.PolicyRef{
			Name:      policy.Name,
			Namespace: policy.Namespace,
		},
		Type: setting.ResourceTypeSystem,
	}, nil
}
