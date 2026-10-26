package mongodb

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func TestListSummariesByWorkflowJobs(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("batch exact pairs and preserve multiple reports", func(mt *mtest.T) {
		coll := &CustomWorkflowTestReportColl{Collection: mt.Coll}
		ns := mt.DB.Name() + "." + mt.Coll.Name()
		mt.AddMockResponses(mtest.CreateCursorResponse(0, ns, mtest.FirstBatch,
			bson.D{{Key: "task_id", Value: int64(1)}, {Key: "job_name", Value: "unittest"}, {Key: "zadig_test_name", Value: "first"}},
			bson.D{{Key: "task_id", Value: int64(2)}, {Key: "job_name", Value: "integration"}},
			bson.D{{Key: "task_id", Value: int64(1)}, {Key: "job_name", Value: "unittest"}, {Key: "zadig_test_name", Value: "second"}},
		))
		reports, err := coll.ListSummariesByWorkflowJobs("workflow", []WorkflowTestReportKey{
			{TaskID: 1, JobName: "UnitTest"},
			{TaskID: 1, JobName: "unittest"},
			{TaskID: 2, JobName: "integration"},
			{TaskID: 3, JobName: "unittest"},
		})
		if err != nil {
			mt.Fatal(err)
		}
		first := reports[WorkflowTestReportKey{TaskID: 1, JobName: "unittest"}]
		if len(first) != 2 || first[0].ZadigTestName != "first" || first[1].ZadigTestName != "second" || len(reports[WorkflowTestReportKey{TaskID: 2, JobName: "integration"}]) != 1 || len(reports[WorkflowTestReportKey{TaskID: 3, JobName: "unittest"}]) != 0 {
			mt.Fatalf("reports mapped to wrong task/job: %#v", reports)
		}
		events := mt.GetAllStartedEvents()
		if len(events) != 1 {
			mt.Fatalf("commands=%d, want one batch find", len(events))
		}
		command := events[0].Command
		if command.Lookup("projection").Document().Lookup("test_cases").Int32() != 0 {
			mt.Fatal("test case details must be excluded")
		}
		filter := command.Lookup("filter").Document()
		if filter.Lookup("workflow_name").StringValue() != "workflow" {
			mt.Fatalf("missing workflow scope: %v", filter)
		}
		pairs, err := filter.Lookup("$or").Array().Values()
		if err != nil || len(pairs) != 3 {
			mt.Fatalf("expected three distinct task/job pairs: %v, %v", pairs, err)
		}
		want := []WorkflowTestReportKey{{TaskID: 1, JobName: "unittest"}, {TaskID: 2, JobName: "integration"}, {TaskID: 3, JobName: "unittest"}}
		for i, pair := range pairs {
			doc := pair.Document()
			if doc.Lookup("task_id").Int64() != want[i].TaskID || doc.Lookup("job_name").StringValue() != want[i].JobName {
				mt.Fatalf("unexpected pair: %v", doc)
			}
		}
	})
	mt.Run("no test jobs skips database", func(mt *mtest.T) {
		coll := &CustomWorkflowTestReportColl{Collection: mt.Coll}
		reports, err := coll.ListSummariesByWorkflowJobs("workflow", nil)
		if err != nil || len(reports) != 0 || len(mt.GetAllStartedEvents()) != 0 {
			mt.Fatalf("empty jobs queried database: reports=%v, err=%v", reports, err)
		}
	})
	mt.Run("query failure is returned", func(mt *mtest.T) {
		coll := &CustomWorkflowTestReportColl{Collection: mt.Coll}
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 2, Message: "invalid query"}))
		if _, err := coll.ListSummariesByWorkflowJobs("workflow", []WorkflowTestReportKey{{TaskID: 1, JobName: "unittest"}}); err == nil {
			mt.Fatal("expected database error")
		}
	})
	mt.Run("detail query still returns test cases", func(mt *mtest.T) {
		coll := &CustomWorkflowTestReportColl{Collection: mt.Coll}
		ns := mt.DB.Name() + "." + mt.Coll.Name()
		mt.AddMockResponses(mtest.CreateCursorResponse(0, ns, mtest.FirstBatch,
			bson.D{{Key: "test_cases", Value: bson.A{bson.D{}}}},
		))
		reports, err := coll.ListByWorkflowJobName("workflow", "UnitTest", 1)
		if err != nil || len(reports) != 1 || len(reports[0].TestCases) != 1 {
			mt.Fatalf("detail data lost: reports=%v, err=%v", reports, err)
		}
		if mt.GetStartedEvent().Command.Lookup("projection").Type != 0 {
			mt.Fatal("detail query must not exclude fields")
		}
	})
}
