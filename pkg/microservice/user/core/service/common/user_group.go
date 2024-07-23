package common

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/config"
	userconfig "github.com/koderover/zadig/v2/pkg/microservice/user/config"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/orm"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

var allUserGroupID string

func GetAllUserGroup() (string, error) {
	if allUserGroupID != "" {
		return allUserGroupID, nil
	}

	group, err := orm.GetAllUserGroup(repository.DB)
	if err != nil {
		log.Errorf("failed to get all-user group, error: %s", err)
		return "", err
	}

	allUserGroupID = group.GroupID
	return allUserGroupID, nil
}

// GetUserGroupByUID list all group IDs the given user with [uid] with cache
func GetUserGroupByUID(uid string) ([]string, error) {
	userGroupKey := fmt.Sprintf(userconfig.UserGroupCacheKeyFormat, uid)
	userCache := cache.NewRedisCache(config.RedisCommonCacheTokenDB())

	// check if the cache has been set
	exists, err := userCache.Exists(userGroupKey)
	if err == nil && exists {
		resp, err2 := userCache.ListSetMembers(userGroupKey)
		if err2 == nil {
			// if we got the data from cache, simply return it
			return resp, nil
		}
	}

	// if no cache is set or anything wrong happens to the cache request, search for the database
	groups, err := orm.ListUserGroupByUID(uid, repository.DB)
	if err != nil {
		log.Errorf("failed to list groups by uid: %s from database, error: %s", uid, err)
		return nil, fmt.Errorf("failed to list groups by uid: %s from database, error: %s", uid, err)
	}

	resp := make([]string, 0)
	for _, group := range groups {
		resp = append(resp, group.GroupID)
	}

	err = userCache.AddElementsToSet(userGroupKey, resp, setting.CacheExpireTime)
	if err != nil {
		// nothing should be returned since setting data into cache does not affect final result
		log.Warnf("failed to add group IDs into user cache, error: %s", err)
	}

	return resp, nil
}
