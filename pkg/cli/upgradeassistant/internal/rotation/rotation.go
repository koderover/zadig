// Package rotation performs a one-shot, offline migration. Cross-database and
// remote writes are not atomic; restore the maintenance backup after any failure.
package rotation

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	common "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	codehost "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/models"
)

type Runner struct {
	Mongo               *mongo.Database
	SQL                 *sql.DB
	Redis               *redis.Client
	Keys                *Keys
	Proxy               string
	CallbackURL         string
	AllowOrphanWebhooks bool
}

type Counts struct {
	Total     int `json:"total"`
	Validated int `json:"validated"`
	Succeeded int `json:"succeeded"`
	Skipped   int `json:"skipped"`
	Failed    int `json:"failed"`
}

type Report struct {
	DryRun            bool               `json:"dry_run"`
	Phase             string             `json:"phase"`
	Records           map[string]*Counts `json:"records"`
	ManualWebhooks    []ManualWebhook    `json:"manual_webhooks"`
	LegacyStorageURIs int64              `json:"legacy_storage_uri_records_skipped"`
}

type ManualWebhook struct {
	HookID      string `json:"hook_id,omitempty"`
	Repository  string `json:"repository"`
	CallbackURL string `json:"callback_url"`
}

type change struct {
	kind, field string
	id          primitive.ObjectID
	uid         string
	old, next   string
}

func (r *Runner) Run(ctx context.Context, dry bool, out io.Writer) (retErr error) {
	report := Report{DryRun: dry, Phase: "preflight", Records: map[string]*Counts{}, ManualWebhooks: []ManualWebhook{}}
	for _, kind := range []string{"s3storage", "k8s_cluster", "user_mfa", "mfa_challenges", "webhook"} {
		report.Records[kind] = &Counts{}
	}
	defer func() {
		if err := json.NewEncoder(out).Encode(report); err != nil && retErr == nil {
			retErr = fmt.Errorf("cannot write rotation report")
		}
	}()

	var changes []change
	for _, kind := range []string{"s3storage", "k8s_cluster"} {
		cursor, err := r.Mongo.Collection(kind).Find(ctx, bson.M{})
		if err != nil {
			return fmt.Errorf("cannot scan %s", kind)
		}
		for cursor.Next(ctx) {
			var id primitive.ObjectID
			var cipher, hash, field string
			if kind == "s3storage" {
				var record common.S3Storage
				if err := cursor.Decode(&record); err != nil {
					cursor.Close(ctx)
					return fmt.Errorf("cannot decode s3storage record")
				}
				id, cipher, field = record.ID, record.EncryptedSk, "encryptedSk"
			} else {
				var record common.K8SCluster
				if err := cursor.Decode(&record); err != nil {
					cursor.Close(ctx)
					return fmt.Errorf("cannot decode k8s_cluster record")
				}
				id, cipher, hash, field = record.ID, record.AgentTokenEnc, record.AgentTokenHash, "agent_token_enc"
			}
			counts := report.Records[kind]
			counts.Total++
			if cipher == "" {
				counts.Skipped++
				continue
			}
			next, err := r.Keys.reencrypt(cipher, kind, hash)
			if err != nil {
				counts.Failed++
				cursor.Close(ctx)
				return fmt.Errorf("%s record %s: %w", kind, id.Hex(), err)
			}
			counts.Validated++
			changes = append(changes, change{kind: kind, field: field, id: id, old: cipher, next: next})
		}
		err = cursor.Err()
		cursor.Close(ctx)
		if err != nil {
			return fmt.Errorf("cannot finish %s scan", kind)
		}
		if kind == "s3storage" && report.Records[kind].Total == 0 {
			return fmt.Errorf("s3storage is empty; refusing rotation")
		}
	}
	rows, err := r.SQL.QueryContext(ctx, "SELECT uid, COALESCE(secret_cipher, '') FROM user_mfa")
	if err != nil {
		return fmt.Errorf("cannot scan user_mfa")
	}
	for rows.Next() {
		var uid, cipher string
		if err := rows.Scan(&uid, &cipher); err != nil {
			rows.Close()
			return fmt.Errorf("cannot decode user_mfa record")
		}
		counts := report.Records["user_mfa"]
		counts.Total++
		if cipher == "" {
			counts.Skipped++
			continue
		}
		next, err := r.Keys.reencrypt(cipher, "user_mfa", "")
		if err != nil {
			counts.Failed++
			rows.Close()
			return fmt.Errorf("user_mfa uid %s: %w", uid, err)
		}
		counts.Validated++
		changes = append(changes, change{kind: "user_mfa", uid: uid, old: cipher, next: next})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return fmt.Errorf("cannot finish user_mfa scan")
	}

	var challenges []string
	iterator := r.Redis.Scan(ctx, 0, "mfa:login:challenge:*", 100).Iterator()
	for iterator.Next(ctx) {
		challenges = append(challenges, iterator.Val())
	}
	if iterator.Err() != nil {
		return fmt.Errorf("cannot scan MFA challenges")
	}
	report.Records["mfa_challenges"].Total = len(challenges)
	report.Records["mfa_challenges"].Validated = len(challenges)

	var hosts []*codehost.CodeHost
	cursor, err := r.Mongo.Collection(codehost.CodeHost{}.TableName()).Find(ctx, bson.M{"deleted_at": 0}, options.Find().SetSort(bson.D{{Key: "id", Value: 1}}))
	if err != nil {
		return fmt.Errorf("cannot scan code hosts")
	}
	err = cursor.All(ctx, &hosts)
	if err != nil {
		return fmt.Errorf("cannot decode code hosts")
	}
	var hooks []*common.WebHook
	cursor, err = r.Mongo.Collection(common.WebHook{}.TableName()).Find(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("cannot scan webhooks")
	}
	if err := cursor.All(ctx, &hooks); err != nil {
		return fmt.Errorf("cannot decode webhooks")
	}
	var actions []*hookAction
	for _, hook := range hooks {
		counts := report.Records["webhook"]
		counts.Total++
		var action *hookAction
		if hook.HookID != "" {
			var err error
			action, err = prepareHookForHosts(ctx, hook, hosts, r.Proxy, r.AllowOrphanWebhooks)
			if err != nil {
				counts.Failed++
				return err
			}
		}
		if action == nil {
			counts.Skipped++
			report.ManualWebhooks = append(report.ManualWebhooks, ManualWebhook{Repository: hook.Address + "/" + hook.Owner + "/" + hook.Repo, CallbackURL: r.CallbackURL, HookID: hook.HookID})
			continue
		}
		actions = append(actions, action)
		counts.Validated++
	}
	// Historic pipeline storage URIs are intentionally not rewritten.
	for _, collection := range []string{"pipeline_task_v2", "pipeline_queue"} {
		n, err := r.Mongo.Collection(collection).CountDocuments(ctx, bson.M{"storage_uri": bson.M{"$exists": true, "$nin": bson.A{"", nil}}})
		if err != nil {
			return fmt.Errorf("cannot count historical storage URIs")
		}
		report.LegacyStorageURIs += n
	}
	if dry {
		report.Phase = "validated"
		return nil
	}

	report.Phase = "writing"
	for _, item := range changes {
		var matched int64
		var err error
		if item.kind == "user_mfa" {
			var result sql.Result
			result, err = r.SQL.ExecContext(ctx, "UPDATE user_mfa SET secret_cipher = ? WHERE uid = ? AND BINARY secret_cipher = ?", item.next, item.uid, item.old)
			if err == nil {
				matched, err = result.RowsAffected()
			}
		} else {
			var result *mongo.UpdateResult
			result, err = r.Mongo.Collection(item.kind).UpdateOne(ctx, bson.M{"_id": item.id, item.field: item.old}, bson.M{"$set": bson.M{item.field: item.next}})
			if err == nil {
				matched = result.MatchedCount
			}
		}
		if err != nil || matched != 1 {
			report.Records[item.kind].Failed++
			return fmt.Errorf("%s update failed or source record changed; keep maintenance mode and restore the final backup before retrying", item.kind)
		}
		report.Records[item.kind].Succeeded++
	}
	for _, key := range challenges {
		if err := r.Redis.Del(ctx, key).Err(); err != nil {
			report.Records["mfa_challenges"].Failed++
			return fmt.Errorf("cannot delete MFA challenge; restore the final backup before retrying")
		}
		report.Records["mfa_challenges"].Succeeded++
	}
	for _, action := range actions {
		if err := action.update(r.Keys.webhookSecret); err != nil {
			report.Records["webhook"].Failed++
			return err
		}
		report.Records["webhook"].Succeeded++
	}
	report.Phase = "completed"
	return nil
}
