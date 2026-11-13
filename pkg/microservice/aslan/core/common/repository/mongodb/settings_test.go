package mongodb

import (
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func TestWorkflowTaskListRefreshInterval(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	for _, tc := range []struct {
		name string
		doc  bson.D
		want int64
	}{
		{name: "existing installation", doc: bson.D{}, want: 3},
		{name: "zero interval", doc: bson.D{{Key: "workflow_task_list_refresh_interval", Value: int64(0)}}, want: 3},
		{name: "null interval", doc: bson.D{{Key: "workflow_task_list_refresh_interval", Value: nil}}, want: 3},
		{name: "negative interval", doc: bson.D{{Key: "workflow_task_list_refresh_interval", Value: int64(-1)}}, want: 3},
		{name: "configured interval", doc: bson.D{{Key: "workflow_task_list_refresh_interval", Value: int64(10)}}, want: 10},
	} {
		mt.Run(tc.name, func(mt *mtest.T) {
			coll := &SystemSettingColl{Collection: mt.Coll}
			for _, get := range []func() (int64, error){
				func() (int64, error) {
					settings, err := coll.Get()
					return settings.WorkflowTaskListRefreshInterval, err
				},
				func() (int64, error) {
					settings, err := coll.GetByID()
					if err != nil {
						return 0, err
					}
					return settings.WorkflowTaskListRefreshInterval, nil
				},
			} {
				mt.AddMockResponses(mtest.CreateCursorResponse(0, mt.DB.Name()+"."+mt.Coll.Name(), mtest.FirstBatch, tc.doc))
				got, err := get()
				if err != nil || got != tc.want {
					t.Fatalf("interval = %d, err = %v; want %d", got, err, tc.want)
				}
			}
		})
	}

	mt.Run("save interval", func(mt *mtest.T) {
		coll := &SystemSettingColl{Collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateSuccessResponse(bson.E{Key: "n", Value: 1}, bson.E{Key: "nModified", Value: 1}))
		if err := coll.UpdateWorkflowTaskListRefreshInterval(10); err != nil {
			t.Fatal(err)
		}
		update := mt.GetStartedEvent().Command.Lookup("updates").Array().Index(0).Value().Document().Lookup("u").Document()
		if got := update.Lookup("$set").Document().Lookup("workflow_task_list_refresh_interval").Int64(); got != 10 {
			t.Fatalf("saved interval = %d; want 10", got)
		}
	})

	for _, tc := range []struct {
		name    string
		matched int
		wantErr error
	}{
		{name: "settings document missing", matched: 0, wantErr: mongo.ErrNoDocuments},
		{name: "interval unchanged", matched: 1},
	} {
		mt.Run(tc.name, func(mt *mtest.T) {
			coll := &SystemSettingColl{Collection: mt.Coll}
			mt.AddMockResponses(mtest.CreateSuccessResponse(bson.E{Key: "n", Value: tc.matched}, bson.E{Key: "nModified", Value: 0}))
			if err := coll.UpdateWorkflowTaskListRefreshInterval(10); !errors.Is(err, tc.wantErr) {
				t.Fatalf("update error = %v; want %v", err, tc.wantErr)
			}
		})
	}
}
