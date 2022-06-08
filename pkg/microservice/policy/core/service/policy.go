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
	"strings"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/service"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/label"
)

type Policy struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	UpdateTime  int64   `json:"update_time"`
	Rules       []*Rule `json:"rules,omitempty"`
}

type ListPolicyResp struct {
	Policies []Policy `json:"policies"`
}

func CreatePolicy(ns string, policy *Policy, _ *zap.SugaredLogger) error {
	obj := &models.Policy{
		Name:      policy.Name,
		Namespace: ns,
		Type:      setting.ResourceTypeSystem,
	}

	for _, r := range policy.Rules {
		obj.Rules = append(obj.Rules, &models.Rule{
			Verbs:           r.Verbs,
			Kind:            r.Kind,
			Resources:       r.Resources,
			MatchAttributes: r.MatchAttributes,
		})
	}
	return mongodb.NewPolicyColl().Create(obj)
}

func CreatePolicies(ns string, policies []*Policy, _ *zap.SugaredLogger) error {
	var objs []*models.Policy
	for _, policy := range policies {
		obj := &models.Policy{
			Name:        policy.Name,
			Namespace:   ns,
			Description: policy.Description,
			UpdateTime:  policy.UpdateTime,
			Type:        setting.ResourceTypeSystem,
		}

		for _, r := range policy.Rules {
			obj.Rules = append(obj.Rules, &models.Rule{
				Verbs:           r.Verbs,
				Kind:            r.Kind,
				Resources:       r.Resources,
				MatchAttributes: r.MatchAttributes,
			})
		}
		objs = append(objs, obj)
	}
	return mongodb.NewPolicyColl().BulkCreate(objs)
}

func UpdatePolicy(ns string, policy *Policy, log *zap.SugaredLogger) error {
	obj := &models.Policy{
		Name:        policy.Name,
		Namespace:   ns,
		UpdateTime:  policy.UpdateTime,
		Description: policy.Description,
	}

	for _, r := range policy.Rules {
		obj.Rules = append(obj.Rules, &models.Rule{
			Verbs:           r.Verbs,
			Kind:            r.Kind,
			Resources:       r.Resources,
			MatchAttributes: r.MatchAttributes,
		})
	}
	return mongodb.NewPolicyColl().UpdatePolicy(obj)
}

func UpdateOrCreatePolicy(ns string, policy *Policy, _ *zap.SugaredLogger) error {
	obj := &models.Policy{
		Name:        policy.Name,
		Namespace:   ns,
		UpdateTime:  policy.UpdateTime,
		Description: policy.Description,
	}

	for _, r := range policy.Rules {
		obj.Rules = append(obj.Rules, &models.Rule{
			Verbs:           r.Verbs,
			Kind:            r.Kind,
			Resources:       r.Resources,
			MatchAttributes: r.MatchAttributes,
		})
	}
	return mongodb.NewPolicyColl().UpdateOrCreate(obj)
}

func ListPolicies(projectName string, _ *zap.SugaredLogger) ([]*Policy, error) {
	var policies []*Policy
	projectPolicies, err := mongodb.NewPolicyColl().ListBy(projectName)
	if err != nil {
		return nil, err
	}
	for _, v := range projectPolicies {
		policies = append(policies, &Policy{
			Name:        v.Name,
			Description: v.Description,
			UpdateTime:  v.UpdateTime,
		})
	}
	return policies, nil
}

func GetPolicies(names string, log *zap.SugaredLogger) ([]*Policy, error) {
	splitedNames := strings.Split(names, ",")
	policies, err := mongodb.NewPolicyColl().GetByNames(splitedNames)
	if err != nil {
		return nil, err
	}
	var res []*Policy
	for _, policy := range policies {
		policy, err := buildPolicy(policy)
		if err != nil {
			log.Errorf("buildPolicy error:%s", err)
			return nil, err
		}
		res = append(res, policy)
	}

	return res, nil
}

func buildPolicy(r *models.Policy) (*Policy, error) {
	res := &Policy{
		Name: r.Name,
	}
	var labels []label.Label
	labelSet := sets.NewString()
	for _, ru := range r.Rules {
		res.Rules = append(res.Rules, &Rule{
			Verbs:           ru.Verbs,
			Kind:            ru.Kind,
			Resources:       ru.Resources,
			MatchAttributes: ru.MatchAttributes,
		})
		for _, ma := range ru.MatchAttributes {
			labelString := service.BuildLabelString(ma.Key, ma.Value)
			if !labelSet.Has(labelString) {
				labelSet.Insert(labelString)
				labels = append(labels, label.Label{
					Key:   ma.Key,
					Value: ma.Value,
				})
			}
		}
	}
	req := label.ListResourcesByLabelsReq{
		LabelFilters: labels,
	}
	labelClient := label.New()
	resp, err := labelClient.ListResourcesByLabels(req)
	if err != nil {
		return nil, err
	}
	for i, rule := range res.Rules {
		var relatedResources []string
		for _, ma := range rule.MatchAttributes {
			labelString := service.BuildLabelString(ma.Key, ma.Value)
			if resources, ok := resp.Resources[labelString]; ok {
				for _, resource := range resources {
					if resource.Type == rule.Resources[0] {
						relatedResources = append(relatedResources, resource.Name)
					}
				}
			}
		}
		res.Rules[i].RelatedResources = relatedResources
	}
	return res, nil
}

func GetPolicy(ns, name string, log *zap.SugaredLogger) (*Policy, error) {
	r, found, err := mongodb.NewPolicyColl().Get(ns, name)
	if err != nil {
		return nil, err
	} else if !found {
		log.Errorf("policy %s not found", name)
		return nil, fmt.Errorf("policy %s not found", name)
	}
	res, err := buildPolicy(r)
	if err != nil {
		log.Errorf("buildPolicy error:%s", err)
		return nil, err
	}
	return res, nil
}

func DeletePolicy(name string, projectName string, logger *zap.SugaredLogger) error {
	err := mongodb.NewPolicyColl().Delete(name, projectName)
	if err != nil {
		logger.Errorf("Failed to delete policy %s in project %s, err: %s", name, projectName, err)
		return err
	}

	return mongodb.NewPolicyBindingColl().DeleteByPolicy(name, projectName)
}

func DeletePolicies(names []string, projectName string, logger *zap.SugaredLogger) error {

	err := mongodb.NewPolicyColl().DeleteMany(names, projectName)
	if err != nil {
		logger.Errorf("Failed to delete policies %s in project %s, err: %s", names, projectName, err)
		return err
	}

	return mongodb.NewPolicyBindingColl().DeleteByPolicies(names, projectName)
}

func ListUserAllPoliciesByPolicyBindings(policyBindings []*models.PolicyBinding) ([]*models.Policy, error) {
	var policies []*models.Policy
	for _, v := range policyBindings {
		tmpRoles, err := mongodb.NewPolicyColl().ListBySpaceAndName(v.PolicyRef.Namespace, v.PolicyRef.Name)
		if err != nil {
			continue
		}
		policies = append(policies, tmpRoles...)
	}
	return policies, nil
}
