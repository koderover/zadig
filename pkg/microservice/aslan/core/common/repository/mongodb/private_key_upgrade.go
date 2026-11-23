package mongodb

import (
	"context"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/agentupgrade"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (c *PrivateKeyColl) UpdateAgentFields(id primitive.ObjectID, fields bson.M) error {
	_, err := c.UpdateOne(context.TODO(), bson.M{"_id": id}, bson.M{"$set": fields})
	return err
}

func (c *PrivateKeyColl) UpdateAgentHeartbeat(id primitive.ObjectID, fields bson.M) error {
	fields["status"] = setting.VMNormal
	_, err := c.UpdateOne(context.TODO(), bson.M{"_id": id, "status": bson.M{"$ne": setting.VMOffline}}, bson.M{"$set": fields})
	return err
}

func (c *PrivateKeyColl) CreateAgentUpgrade(id primitive.ObjectID, version string, task *agentupgrade.Task) (*models.PrivateKey, error) {
	var vm models.PrivateKey
	query := bson.M{
		"_id": id, "status": setting.VMNormal, "agent.upgrade_supported": true,
		"agent.agent_version":       version,
		"agent.last_heartbeat_time": bson.M{"$gte": time.Now().Unix() - int64(setting.AgentDefaultHeartbeatTimeout)},
		"$or": bson.A{
			bson.M{"agent.upgrade": nil},
			bson.M{"agent.upgrade.status": bson.M{"$in": bson.A{agentupgrade.Succeeded, agentupgrade.Failed, agentupgrade.RolledBack}}},
		},
	}
	err := c.FindOneAndUpdate(context.TODO(), query, bson.M{"$set": bson.M{"agent.upgrade": task}}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&vm)
	return &vm, err
}

func (c *PrivateKeyColl) ReportAgentUpgrade(id primitive.ObjectID, task *agentupgrade.Task, report *agentupgrade.Report) error {
	now := time.Now().Unix()
	fields := bson.M{"agent.upgrade.status": report.Status, "agent.upgrade.error": report.Error, "agent.upgrade.update_time": now}
	if report.Status.Terminal() {
		fields["agent.upgrade.finish_time"] = now
	}
	_, err := c.UpdateOne(context.TODO(), bson.M{"_id": id, "agent.upgrade.id": task.ID, "agent.upgrade.status": task.Status}, bson.M{"$set": fields})
	return err
}
