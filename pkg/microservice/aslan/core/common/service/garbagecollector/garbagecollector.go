package garbagecollector

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/mongodb"
	labeldb "github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/mongodb"
)

func DeleteRelatedResource(projectName string, resourceType config.ResourceType, resource, project string, log *zap.SugaredLogger) error {
	// delete the labelBindings by resources
	resources := []labeldb.Resource{
		{
			Name:        resource,
			ProjectName: project,
			Type:        string(resourceType),
		},
	}
	err := mongodb.NewLabelBindingColl().DeleteByResources(mongodb.DeleteLabelBindingsByResource{Resources: resources})
	if err != nil {
		log.Errorf("delete releated labelbindings err:%v", err)
		return err
	}
	// if the resource's label doesn't have any labelBindings , delete the label  and update the policy

	// 查看本项目下的所有labels
	labels, err := mongodb.NewLabelColl().ListByProjectName(projectName)
	if err != nil {
		return err
	}

	// 对于每一个label,查看当前项目下是否还有相应的绑定，如果没有，删除这个label
	for _, label := range labels {
		labelBindings, err := mongodb.NewLabelBindingColl().ListByOpt(&mongodb.LabelBindingCollFindOpt{
			LabelID:     label.ID.Hex(),
			ProjectName: project,
		})
		if err != nil {
			return err
		}
		if len(labelBindings) == 0 {
			// 删除这个label
			if err := mongodb.NewLabelColl().Delete(label.ID.Hex()); err != nil {
				return err
			}
		}
	}
	return nil
}
