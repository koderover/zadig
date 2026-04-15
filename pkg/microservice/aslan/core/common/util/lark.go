package util

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	larkservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/lark"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/lark"
	"k8s.io/apimachinery/pkg/util/sets"
)

func ConvertLarkUserGroupToUser(larkApprovalID string, groups []*models.LarkApprovalGroup) ([]*lark.UserInfo, error) {
	userSet := sets.NewString()
	users := make([]*lark.UserInfo, 0)
	for _, group := range groups {
		userGroup, err := larkservice.GetLarkUserGroup(larkApprovalID, group.GroupID)
		if err != nil {
			return nil, fmt.Errorf("failed to get lark user group: %s", err)
		}

		if userGroup.MemberUserCount > 0 {
			userInfos, err := larkservice.GetLarkUserGroupMembersInfo(larkApprovalID, group.GroupID, "user", setting.LarkUserOpenID, "")
			if err != nil {
				return nil, fmt.Errorf("failed to get lark department user infos: %s", err)
			}

			for _, user := range userInfos {
				if !userSet.Has(user.ID) {
					users = append(users, user)
					userSet.Insert(user.ID)
				}
			}
		}

		if userGroup.MemberDepartmentCount > 0 {
			userInfos, err := larkservice.GetLarkUserGroupMembersInfo(larkApprovalID, group.GroupID, "department", setting.LarkDepartmentID, "")
			if err != nil {
				return nil, fmt.Errorf("failed to get lark department user infos: %s", err)
			}

			for _, user := range userInfos {
				if !userSet.Has(user.ID) {
					users = append(users, user)
					userSet.Insert(user.ID)
				}
			}
		}
	}
	return users, nil
}
