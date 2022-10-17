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
	"time"

	"github.com/koderover/zadig/pkg/tool/errors"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/log"
	"go.uber.org/zap"
)

type VariableSetFindOption struct {
	ID      string `json:"id"`
	PerPage int    `json:"perPage"`
	Page    int    `json:"page"`
}

type VariableSetListResp struct {
	VariableSetList []*commonmodels.VariableSet `json:"variable_set_list"`
	Total           int64                       `json:"total"`
}

type CreateVariableSetRequest struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	ProjectName  string `json:"project_name"`
	VariableYaml string `json:"variable_yaml"`
	UserName     string
}

func CreateVariableSet(args *CreateVariableSetRequest) error {
	modelData := &commonmodels.VariableSet{
		Name:         args.Name,
		Description:  args.Description,
		ProjectName:  args.ProjectName,
		VariableYaml: args.VariableYaml,
		CreatedAt:    time.Now().Unix(),
		CreatedBy:    args.UserName,
		UpdatedAt:    time.Now().Unix(),
		UpdatedBy:    args.UserName,
	}
	if err := commonrepo.NewVariableSetColl().Create(modelData); err != nil {
		log.Errorf("CreateVariableSet err:%v", err)
		return errors.ErrCreateVariableSet.AddErr(err)
	}
	return nil
}

func UpdateVariableSet(args *CreateVariableSetRequest) error {
	modelData := &commonmodels.VariableSet{
		Name:         args.Name,
		Description:  args.Description,
		ProjectName:  args.ProjectName,
		VariableYaml: args.VariableYaml,
		UpdatedAt:    time.Now().Unix(),
		UpdatedBy:    args.UserName,
	}
	if err := commonrepo.NewVariableSetColl().Update(args.ID, modelData); err != nil {
		log.Errorf("UpdateVariableSet err:%v", err)
		return errors.ErrEditVariableSet.AddErr(err)
	}
	return nil
}

func GetVariableSet(idStr string, log *zap.SugaredLogger) (*commonmodels.VariableSet, error) {
	variableset, err := commonrepo.NewVariableSetColl().Find(&commonrepo.VariableSetFindOption{
		ID: idStr,
	})
	if err != nil {
		log.Errorf("GetVariableSet err:%v", err)
		return nil, errors.ErrGetVariableSet.AddErr(err)
	}
	return variableset, nil
}

func ListVariableSets(option *VariableSetFindOption, log *zap.SugaredLogger) (*VariableSetListResp, error) {
	if option.Page < 1 {
		option.Page = 1
	}
	if option.PerPage < 1 {
		option.PerPage = 20
	}
	count, variablesets, err := commonrepo.NewVariableSetColl().List(&commonrepo.VariableSetFindOption{
		Page:    option.Page,
		PerPage: option.PerPage,
	})
	if err != nil {
		log.Errorf("ListVariableSets err:%v", err)
		return nil, errors.ErrListVariableSets.AddErr(err)
	}
	return &VariableSetListResp{
		VariableSetList: variablesets,
		Total:           count,
	}, nil
}

func DeleteVariableSet(id, projectName string, log *zap.SugaredLogger) error {
	err := commonrepo.NewVariableSetColl().Delete(id)
	if err != nil {
		log.Errorf("DeleteVariableSet err:%v", err)
		return errors.ErrDeleteVariableSet.AddErr(err)
	}
	return nil
}
