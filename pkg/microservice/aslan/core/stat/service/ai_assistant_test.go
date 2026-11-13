package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"io"
	"net/http"
	"strings"

	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models"
	statorm "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/orm"
)

var reportColumns = []string{"key", "runs", "completed", "terminal", "in", "out", "elapsed", "completed_elapsed", "positive", "negative", "users"}

func mockAIAssistantStatsStore(t *testing.T) (*statorm.AIAssistantStatsStore, sqlmock.Sqlmock) {
	t.Helper()
	log.Init(&log.Config{Level: "error"})
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	store, err := statorm.NewAIAssistantStatsStore(db)
	if err != nil {
		t.Fatal(err)
	}
	originalClient := newAIAssistantStatsUserClient
	newAIAssistantStatsUserClient = func() *user.Client {
		client := &user.Client{Client: httpclient.New(httpclient.SetHostURL("http://user/api/v1"))}
		client.SetTransport(statsUserTransport(func(r *http.Request) (*http.Response, error) {
			if deadline, ok := r.Context().Deadline(); !ok || time.Until(deadline) > 3*time.Second {
				t.Error("user 查询必须共用请求超时预算")
			}
			body := `{"total_user":10}`
			switch r.URL.Path {
			case "/api/v1/users/count":
			case "/api/v1/users/search":
				body = `{"users":[{"uid":"user-1","name":"张三"}]}`
			default:
				t.Errorf("意外调用: %s", r.URL.Path)
			}
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
		}))
		return client
	}
	t.Cleanup(func() { newAIAssistantStatsUserClient = originalClient })
	original := getAIAssistantStatsStore
	getAIAssistantStatsStore = func() (*statorm.AIAssistantStatsStore, error) { return store, nil }
	t.Cleanup(func() {
		getAIAssistantStatsStore = original
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
		db.Close()
	})
	return store, mock
}

func TestReportRangeAndSavingsBoundaries(t *testing.T) {
	zone := time.FixedZone("Asia/Shanghai", 8*3600)
	now := time.Date(2026, 11, 2, 1, 0, 0, 0, zone)
	from, to, err := reportRange("", "", now)
	if err != nil || from.Format("2006-01-02") != "2026-10-04" || to.Format("2006-01-02") != "2026-11-02" {
		t.Fatalf("默认报告范围错误: %v", err)
	}

	for _, dates := range [][2]string{
		{"2025-11-02", "2026-11-02"},
		{"2026-11-03", "2026-11-03"},
		{"2026-11-02", "2026-11-01"},
		{"bad", ""},
	} {
		if _, _, err := reportRange(dates[0], dates[1], now); err == nil {
			t.Fatalf("不应接受报告范围: %v", dates)
		}
	}

	saved := estimatedSavedMinutes([]models.AIAssistantUsageReportRow{
		{Key: "workflow_diagnosis", AIAssistantStatsMetrics: models.AIAssistantStatsMetrics{Completed: 2, CompletedElapsedMS: 10 * 60000}},
		{Key: "project_onboarding", AIAssistantStatsMetrics: models.AIAssistantStatsMetrics{Completed: 1, CompletedElapsedMS: 50 * 60000}},
		{Key: "environment_inspection", AIAssistantStatsMetrics: models.AIAssistantStatsMetrics{Completed: 1, CompletedElapsedMS: 60000}},
		{Key: "efficiency_diagnosis", AIAssistantStatsMetrics: models.AIAssistantStatsMetrics{Completed: 1, CompletedElapsedMS: 2 * 60000}},
	})
	if saved != 77 {
		t.Fatalf("仅成功任务计入，负节省应归零: %v", saved)
	}
}

var aiAssistantReportQueries = map[string]func(context.Context, string, string) (any, error){
	"overview": func(ctx context.Context, from, to string) (any, error) {
		return GetAIAssistantStatsOverview(ctx, from, to)
	},
	"trends": func(ctx context.Context, from, to string) (any, error) {
		return GetAIAssistantStatsTrends(ctx, from, to)
	},
	"skills": func(ctx context.Context, from, to string) (any, error) {
		return GetAIAssistantStatsSkills(ctx, from, to)
	},
	"rankings": func(ctx context.Context, from, to string) (any, error) {
		return GetAIAssistantStatsRankings(ctx, from, to)
	},
}

func TestAIAssistantStatsOverviewAndMissingFeedback(t *testing.T) {
	for _, positive := range []int{0, 1} {
		t.Run([]string{"无反馈", "有反馈"}[positive], func(t *testing.T) {
			_, mock := mockAIAssistantStatsStore(t)
			mock.ExpectQuery("SELECT .*FROM ai_usage_daily").WillReturnRows(sqlmock.NewRows(reportColumns).AddRow("overview", 1, 1, 1, 10, 5, 1000, 60000, positive, 0, 1))
			expectStatsModels(mock)
			mock.ExpectQuery("SELECT .*FROM ai_usage_daily").WillReturnRows(sqlmock.NewRows(reportColumns).AddRow("project_onboarding", 1, 1, 1, 10, 5, 1000, 60000, positive, 0, 1))
			mock.ExpectQuery("SELECT .*FROM ai_usage_daily").WillReturnRows(sqlmock.NewRows(reportColumns))
			mock.ExpectQuery("SELECT .*FROM ai_usage_daily").WillReturnRows(sqlmock.NewRows(reportColumns))
			result, err := GetAIAssistantStatsOverview(context.Background(), "", "")
			if err != nil {
				t.Fatal(err)
			}
			if result.Metrics.Completed != 1 || result.EstimatedSavedMinutes != 39 || (result.EstimatedSavedPersonDays == nil) != (positive == 0) {
				t.Fatalf("指标或无反馈口径错误: %+v", result)
			}
			if positive == 1 && *result.EstimatedSavedPersonDays != 39.0/60/8 {
				t.Fatal("节省人天计算错误")
			}
			raw, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]any
			if err := json.Unmarshal(raw, &fields); err != nil {
				t.Fatal(err)
			}
			if _, ok := fields["estimated_saved_person_days"]; !ok {
				t.Fatal("无反馈时应显式返回空人天")
			}
			if fields["items"] != nil || fields["maintenance"] != nil || fields["business_database_pool"] != nil {
				t.Fatal("概览应仅返回独立指标及估算，不返回通用列表或进程状态")
			}
		})
	}
}

func TestAIAssistantStatsResponsesUseExplicitDimensions(t *testing.T) {
	for _, item := range []struct{ kind, dimension, key string }{
		{"trends", "date", "2026-11-01"},
		{"rankings", "user_id", "user-1"},
	} {
		t.Run(item.kind, func(t *testing.T) {
			_, mock := mockAIAssistantStatsStore(t)
			mock.ExpectQuery("SELECT .*FROM ai_usage_daily").WillReturnRows(sqlmock.NewRows(reportColumns).AddRow(item.key, 2, 1, 2, 10, 5, 1000, 60000, 1, 1, 1))
			expectStatsModels(mock)
			response, err := aiAssistantReportQueries[item.kind](context.Background(), "", "")
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(response)
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				Items []map[string]any `json:"items"`
			}
			if err := json.Unmarshal(raw, &result); err != nil {
				t.Fatal(err)
			}
			if len(result.Items) != 1 {
				t.Fatalf("报告行丢失: %s", raw)
			}
			row := result.Items[0]
			if row[item.dimension] != item.key || row["key"] != nil || row["completed_count"] != float64(1) || row["completion_rate"] != 0.5 || row["adoption_rate"] != 0.5 {
				t.Fatalf("维度或指标转换错误: %s", raw)
			}
		})
	}
}

func TestAIAssistantStatsSkillsUsesFourCategories(t *testing.T) {
	_, mock := mockAIAssistantStatsStore(t)
	mock.ExpectQuery("SELECT s.category .*JOIN ai_skill s ON s.id=ai_usage_daily.skill_id.*GROUP BY group_key").WillReturnRows(sqlmock.NewRows(reportColumns).
		AddRow("故障排查", 5, 3, 4, 100, 50, 2000, 1000, 3, 1, 2).
		AddRow("实施辅助", 3, 1, 2, 60, 30, 3000, 1000, 1, 1, 1))
	expectStatsModels(mock)
	result, err := GetAIAssistantStatsSkills(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	for i, category := range []string{"发布辅助", "故障排查", "实施辅助", "巡检诊断"} {
		if len(result.Items) != 4 || result.Items[i].Category != category {
			t.Fatalf("必须按固定顺序返回四类: %+v", result)
		}
	}
	fault := result.Items[1]
	if fault.Completed != 3 || fault.ActiveUsers != 2 || *fault.CompletionRate != 0.75 || *fault.AdoptionRate != 0.75 {
		t.Fatalf("应使用分类聚合的计数、去重用户及比例: %+v", fault)
	}
	if result.Items[0].Completed != 0 || result.Items[0].CompletionRate != nil || result.Items[3].AdoptionRate != nil {
		t.Fatal("无数据分类不能伪造完成任务或比例")
	}
	raw, err := json.Marshal(result.Items[1])
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil || fields["category"] != "故障排查" || fields["skill_id"] != nil {
		t.Fatalf("响应维度应为 category，不再返回 skill_id: %s %v", raw, err)
	}
}

func TestAIAssistantStatsEmptyResponses(t *testing.T) {
	for kind, query := range aiAssistantReportQueries {
		t.Run(kind, func(t *testing.T) {
			_, mock := mockAIAssistantStatsStore(t)
			mock.ExpectQuery("SELECT .*FROM ai_usage_daily").WillReturnRows(sqlmock.NewRows(reportColumns))
			if kind == "overview" {
				for i := 0; i < 2; i++ {
					mock.ExpectQuery("SELECT .*FROM ai_usage_daily").WillReturnRows(sqlmock.NewRows(reportColumns))
				}
				mock.ExpectQuery("SELECT .*FROM ai_usage_daily").WillReturnRows(sqlmock.NewRows(reportColumns))
			}
			response, err := query(context.Background(), "", "")
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(response)
			if err != nil {
				t.Fatal(err)
			}
			var result map[string]any
			if err := json.Unmarshal(raw, &result); err != nil {
				t.Fatal(err)
			}
			if kind == "overview" {
				metrics := result["metrics"].(map[string]any)
				if metrics["completed_count"] != float64(0) || metrics["adoption_rate"] != nil || result["estimated_saved_person_days"] != nil {
					t.Fatalf("空概览不能伪造指标: %s", raw)
				}
			} else if kind == "skills" {
				items := result["items"].([]any)
				if len(items) != 4 {
					t.Fatalf("空报告也必须返回四个分类: %s", raw)
				}
				for _, item := range items {
					row := item.(map[string]any)
					if row["completed_count"] != float64(0) || row["completion_rate"] != nil || row["adoption_rate"] != nil {
						t.Fatalf("无数据分类应返回零计数与空比例: %s", raw)
					}
				}
			} else {
				items, ok := result["items"].([]any)
				if !ok || len(items) != 0 || result["estimated_saved_minutes"] != nil || result["estimated_saved_person_days"] != nil {
					t.Fatalf("空列表应为数组，且无概览字段: %s", raw)
				}
			}
		})
	}
}

func TestAIAssistantStatsDatabaseFailureAndRecovery(t *testing.T) {
	original := getAIAssistantStatsStore
	t.Cleanup(func() { getAIAssistantStatsStore = original })
	calls := 0
	getAIAssistantStatsStore = func() (*statorm.AIAssistantStatsStore, error) { calls++; return nil, errors.New("unavailable") }
	for kind, query := range aiAssistantReportQueries {
		if _, err := query(context.Background(), "bad", ""); !errors.Is(err, ErrAIAssistantStatsRange) || calls != 0 {
			t.Fatalf("%s 的无效日期不能访问数据库", kind)
		}
		if _, err := query(context.Background(), "", ""); !errors.Is(err, ErrAIAssistantStatsUnavailable) {
			t.Fatalf("%s 的数据库故障应只影响统计", kind)
		}
		calls = 0
	}
	_, mock := mockAIAssistantStatsStore(t)
	mock.ExpectQuery("SELECT .*FROM ai_usage_daily").WillReturnError(context.DeadlineExceeded)
	if _, err := GetAIAssistantStatsTrends(context.Background(), "", ""); !errors.Is(err, ErrAIAssistantStatsUnavailable) {
		t.Fatal("读取失败必须报告错误")
	}
	mock.ExpectQuery("SELECT .*FROM ai_usage_daily").WillReturnRows(sqlmock.NewRows(reportColumns))
	if _, err := GetAIAssistantStatsTrends(context.Background(), "", ""); err != nil {
		t.Fatal("查询失败后应允许恢复")
	}
}

func TestAIAssistantStatsFourReportsCanLoadConcurrently(t *testing.T) {
	store, mock := mockAIAssistantStatsStore(t)
	mock.MatchExpectationsInOrder(false)
	for i := 0; i < 7; i++ {
		mock.ExpectQuery("SELECT .*FROM ai_usage_daily").WillDelayFor(100 * time.Millisecond).WillReturnRows(sqlmock.NewRows(reportColumns))
	}
	start := make(chan struct{})
	results := make(chan error, 4)
	var ready, done sync.WaitGroup
	ready.Add(4)
	done.Add(4)
	for _, query := range aiAssistantReportQueries {
		go func(query func(context.Context, string, string) (any, error)) {
			defer done.Done()
			ready.Done()
			<-start
			_, err := query(context.Background(), "", "")
			results <- err
		}(query)
	}
	ready.Wait()
	close(start)
	done.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("四个报告并发加载不应立即拒绝: %v", err)
		}
	}
	stats := store.SQLDB.Stats()
	if stats.MaxOpenConnections != 3 || stats.OpenConnections > 3 || stats.WaitCount == 0 {
		t.Fatalf("应在三个连接内等待后完成: %+v", stats)
	}
}

func TestAIAssistantStatsConnectionWaitUsesQueryDeadline(t *testing.T) {
	store, _ := mockAIAssistantStatsStore(t)
	for i := 0; i < 3; i++ {
		conn, err := store.SQLDB.Conn(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := GetAIAssistantStatsTrends(ctx, "", ""); !errors.Is(err, ErrAIAssistantStatsUnavailable) {
		t.Fatal("等待数据库连接也必须响应请求超时")
	}
}

func TestAIAssistantStatsQueryHasThreeSecondDeadline(t *testing.T) {
	_, mock := mockAIAssistantStatsStore(t)
	mock.ExpectQuery("SELECT .*FROM ai_usage_daily").WillDelayFor(4 * time.Second).WillReturnRows(sqlmock.NewRows(reportColumns))
	if _, err := GetAIAssistantStatsTrends(context.Background(), "", ""); !errors.Is(err, ErrAIAssistantStatsUnavailable) {
		t.Fatal("查询必须受三秒预算限制")
	}
}

var statsModelColumns = []string{"key", "model_id", "model", "in", "out"}

func expectStatsModels(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("SELECT .*model_id,model_name,SUM\\(input_tokens\\)").WillReturnRows(sqlmock.NewRows(statsModelColumns))
}

type statsUserTransport func(*http.Request) (*http.Response, error)

func (f statsUserTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAIAssistantStatsOverviewPreviousPeriodAndUsers(t *testing.T) {
	_, mock := mockAIAssistantStatsStore(t)
	// 当前日和上期日故意采用不同指标，验证范围与估算不串用。
	today := time.Now().In(time.Local).Format("2006-01-02")
	yesterday := time.Now().In(time.Local).AddDate(0, 0, -1).Format("2006-01-02")
	for _, period := range []struct {
		date      string
		completed int64
	}{{today, 2}, {yesterday, 1}} {
		mock.ExpectQuery("SELECT .*FROM ai_usage_daily").WithArgs(period.date, period.date).WillReturnRows(sqlmock.NewRows(reportColumns).AddRow("overview", 2, period.completed, 2, 0, 0, 0, 0, 1, 0, 1))
		mock.ExpectQuery("SELECT .*FROM ai_usage_daily").WithArgs(period.date, period.date).WillReturnRows(sqlmock.NewRows(reportColumns).AddRow("workflow_diagnosis", 2, period.completed, 2, 0, 0, 0, 0, 1, 0, 1))
	}
	result, err := GetAIAssistantStatsOverview(context.Background(), today, today)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalUsers != 10 || result.Metrics.ActiveUsers != 1 || result.Previous == nil || result.Previous.From != yesterday || result.Previous.To != yesterday || result.Metrics.Completed != 2 || result.Previous.Metrics.Completed != 1 || result.EstimatedSavedMinutes != 60 || result.Previous.EstimatedSavedMinutes != 30 {
		t.Fatalf("上期或系统用户数错误: %+v", result)
	}
}

func TestAIAssistantStatsPreviousRangeUsesCalendarDays(t *testing.T) {
	for _, tc := range []struct{ from, to, wantFrom, wantTo string }{
		{"2024-03-01", "2024-03-31", "2024-01-30", "2024-02-29"},
		{"2026-01-01", "2026-01-01", "2025-12-31", "2025-12-31"},
		{"2026-10-30", "2026-11-02", "2026-10-26", "2026-10-29"},
	} {
		from, to := previousStatsRange(models.AIAssistantStatsPeriod{From: tc.from, To: tc.to})
		if from != tc.wantFrom || to != tc.wantTo {
			t.Fatalf("上期错误: %s %s", from, to)
		}
	}
}

func TestAIAssistantStatsPreviousOutsideRetentionIsUnavailable(t *testing.T) {
	_, mock := mockAIAssistantStatsStore(t)
	for i := 0; i < 2; i++ {
		mock.ExpectQuery("SELECT .*FROM ai_usage_daily").WillReturnRows(sqlmock.NewRows(reportColumns))
	}
	oldest := time.Now().In(time.Local).AddDate(0, 0, -364).Format("2006-01-02")
	result, err := GetAIAssistantStatsOverview(context.Background(), oldest, oldest)
	if err != nil || result.Previous != nil {
		t.Fatalf("已清理上期不能伪造零数据: %+v %v", result, err)
	}
}

func TestAIAssistantStatsRankingsNamesAndUserLookupFailure(t *testing.T) {
	_, mock := mockAIAssistantStatsStore(t)
	rows := sqlmock.NewRows(reportColumns)
	for _, id := range []string{"user-1", "deleted-user"} {
		rows.AddRow(id, 1, 0, 0, 0, 0, 0, 0, 0, 0, 1)
	}
	mock.ExpectQuery("SELECT .*FROM ai_usage_daily").WillReturnRows(rows)
	result, err := GetAIAssistantStatsRankings(context.Background(), "", "")
	if err != nil || result.Items[0].UserName != "张三" || result.Items[1].UserName != "deleted-user" {
		t.Fatalf("用户名或已删除用户回退错误: %+v %v", result, err)
	}
	originalClient := newAIAssistantStatsUserClient
	newAIAssistantStatsUserClient = func() *user.Client {
		client := originalClient()
		client.SetTransport(statsUserTransport(func(*http.Request) (*http.Response, error) { return nil, errors.New("user unavailable") }))
		return client
	}
	for _, kind := range []string{"rankings", "overview"} {
		mock.ExpectQuery("SELECT .*FROM ai_usage_daily").WillReturnRows(sqlmock.NewRows(reportColumns).AddRow("user-1", 1, 0, 0, 0, 0, 0, 0, 0, 0, 1))
		if kind == "overview" {
			for i := 0; i < 3; i++ {
				mock.ExpectQuery("SELECT .*FROM ai_usage_daily").WillReturnRows(sqlmock.NewRows(reportColumns))
			}
		}
		if _, err := aiAssistantReportQueries[kind](context.Background(), "", ""); !errors.Is(err, ErrAIAssistantStatsUnavailable) {
			t.Fatalf("%s 的 user 查询失败不能伪造统计: %v", kind, err)
		}
	}
}
