package migrate

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/tool/log"
)

func init() {
	upgradepath.RegisterHandler("1.10.0", "1.11.0", V1100ToV1110)
	upgradepath.RegisterHandler("1.11.0", "1.10.0", V1110ToV1100)
}

func V1100ToV1110() error {
	// iterate all roles ， if the role contains manage env , add debug_pod
	if err := roleAddPodDebug(); err != nil {
		return err
	}

	return nil
}

func V1110ToV1100() error {
	if err := roleDeletePodDebug(); err != nil {
		return err
	}
	return nil
}

func roleAddPodDebug() error {
	log.Info("start to scan role")
	ctx := context.Background()
	var res []*models.Role
	cursor, err := newRoleColl().Find(ctx, bson.M{})
	if err != nil {
		log.Errorf("Failed to find roles, err: %s", err)
		return err
	}
	err = cursor.All(ctx, &res)
	if err != nil {
		log.Errorf("get roles err:%s", err)
		return err
	}
	for _, role := range res {
		log.Infof("start to check role:%s", role.Name)
		toUpdate := false
		for i, rule := range role.Rules {
			verbsSet := sets.NewString(rule.Verbs...)
			if verbsSet.Has("manage_environment") {
				verbsSet.Insert("debug_pod")
				role.Rules[i].Verbs = verbsSet.List()
				toUpdate = true
			}
		}
		if toUpdate {
			// start to update
			log.Infof("find role:%s has verb:manage_environment ,start to update the role\n", role.Name)
			query := bson.M{"name": role.Name}
			change := bson.M{"$set": bson.M{
				"rules": role.Rules,
			}}
			_, err := newRoleColl().UpdateOne(ctx, query, change)
			if err != nil {
				log.Errorf("fail to update,err:%s", err)
				return err
			}
		}
	}
	return nil
}

func roleDeletePodDebug() error {
	ctx := context.Background()
	var res []*models.Role
	cursor, err := newRoleColl().Find(ctx, bson.M{})
	if err != nil {
		log.Errorf("Fail to find roles, err: %s", err)
		return err
	}
	err = cursor.All(ctx, &res)
	if err != nil {
		log.Errorf("get roles err:%s", err)
		return err
	}
	for _, role := range res {
		toUpdate := false
		for i, rule := range role.Rules {
			verbsSet := sets.NewString(rule.Verbs...)
			if verbsSet.Has("debug_pod") {
				verbsSet.Delete("debug_pod")
				role.Rules[i].Verbs = verbsSet.List()
				toUpdate = true
			}
		}
		if toUpdate {
			// start to update
			log.Infof("find role:%s has verb:debug_pod ,start to update the role", role.Name)
			query := bson.M{"name": role.Name}
			change := bson.M{"$set": bson.M{
				"rules": role.Rules,
			}}
			_, err := newRoleColl().UpdateOne(ctx, query, change)
			if err != nil {
				log.Errorf("err update%s\n", err)
				return err
			}
		}
	}
	return nil
}
