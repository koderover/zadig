package jobcontroller

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
	steptypes "github.com/koderover/zadig/v2/pkg/types/step"
)

func TestBuildAIApolloChangeSummariesActions(t *testing.T) {
	tests := []struct {
		name        string
		namespace   commonmodels.ApolloNamespace
		wantAction  commonmodels.ApolloActionType
		wantAdded   []string
		wantUpdated []string
		wantRemoved []string
	}{
		{
			name: "create reports submitted keys as additions",
			namespace: commonmodels.ApolloNamespace{
				Action: commonmodels.ApolloActionCreate, AppID: "app", Namespace: "feature", Type: "properties",
				KeyValList: []*commonmodels.ApolloKV{{Key: "A", Val: "secret-after"}, {Key: "B", Val: "secret-after"}},
			},
			wantAction: commonmodels.ApolloActionCreate, wantAdded: []string{"A", "B"},
		},
		{
			name: "empty namespace remains a create operation",
			namespace: commonmodels.ApolloNamespace{
				Action: commonmodels.ApolloActionCreate, AppID: "app", Namespace: "empty", Type: "properties",
			},
			wantAction: commonmodels.ApolloActionCreate,
		},
		{
			name: "file create retains item-level difference",
			namespace: commonmodels.ApolloNamespace{
				Action: commonmodels.ApolloActionCreate, AppID: "app", Namespace: "feature.yaml", Type: "yaml",
				KeyValList: []*commonmodels.ApolloKV{{Key: "content", Val: "password: secret-after"}},
			},
			wantAction: commonmodels.ApolloActionCreate, wantAdded: []string{"content"},
		},
		{
			name: "update retains target and differences",
			namespace: commonmodels.ApolloNamespace{
				Action: commonmodels.ApolloActionUpdate, AppID: "app", Env: "REGION1", ClusterID: "prod", Namespace: "feature", Type: "properties",
				OriginalConfig: []*commonmodels.ApolloKV{{Key: "A", Val: "secret-before"}, {Key: "B", Val: "secret-before"}},
				KeyValList:     []*commonmodels.ApolloKV{{Key: "A", Val: "secret-after"}, {Key: "C", Val: "secret-after"}},
			},
			wantAction: commonmodels.ApolloActionUpdate, wantAdded: []string{"C"}, wantUpdated: []string{"A"}, wantRemoved: []string{"B"},
		},
		{
			name: "legacy action defaults to update",
			namespace: commonmodels.ApolloNamespace{
				AppID: "app", Env: "REGION1", ClusterID: "default", Namespace: "feature", Type: "properties",
				OriginalConfig: []*commonmodels.ApolloKV{{Key: "A", Val: "secret-before"}},
				KeyValList:     []*commonmodels.ApolloKV{{Key: "A", Val: "secret-after"}},
			},
			wantAction: commonmodels.ApolloActionUpdate, wantUpdated: []string{"A"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &commonmodels.AIReleaseSummaryItem{}
			appendAIReleaseSpecialistTaskDetails(item, &commonmodels.JobTask{
				JobType: string(config.JobApollo),
				Spec: &commonmodels.JobTaskApolloSpec{NamespaceList: []*commonmodels.JobTaskApolloNamespace{
					{ApolloNamespace: tt.namespace},
				}},
			})
			if len(item.ConfigChanges) != 1 {
				t.Fatalf("expected one config summary, got %d", len(item.ConfigChanges))
			}
			got := item.ConfigChanges[0]
			if got.Action != string(tt.wantAction) {
				t.Fatalf("unexpected action: %q", got.Action)
			}
			if got.NamespaceName != tt.namespace.Env || got.Group != tt.namespace.ClusterID || got.DataID != "app/"+tt.namespace.Namespace {
				t.Fatalf("unexpected configuration target: %#v", got)
			}
			if !reflect.DeepEqual(got.AddedFields, tt.wantAdded) || !reflect.DeepEqual(got.UpdatedFields, tt.wantUpdated) || !reflect.DeepEqual(got.RemovedFields, tt.wantRemoved) {
				t.Fatalf("unexpected differences: added=%v updated=%v removed=%v", got.AddedFields, got.UpdatedFields, got.RemovedFields)
			}
			count := len(tt.wantAdded) + len(tt.wantUpdated) + len(tt.wantRemoved)
			if got.ChangedFieldCount != count || got.ContentChanged != (count > 0) || !got.ChangedFieldsAvailable || (got.ChangedFieldsHash != "") != (count > 0) {
				t.Fatalf("unexpected content-change metadata: %#v", got)
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(encoded), "secret-") {
				t.Fatalf("summary contains configuration values: %s", encoded)
			}
		})
	}
}

func TestBuildAIReleaseSpecialistInputWithRulePlanKeepsBuildAndChangeContext(t *testing.T) {
	buildJob := &commonmodels.JobTask{
		Name:       "build-task",
		OriginName: "build-task",
		JobType:    string(config.JobZadigBuild),
		Status:     config.StatusPassed,
		Spec: &commonmodels.JobTaskFreestyleSpec{
			Steps: []*commonmodels.StepTask{
				{
					StepType: config.StepGit,
					Spec: &steptypes.StepGitSpec{
						Repos: []*types.Repository{
							{
								RepoName:      "service-a",
								Branch:        "release/1.0",
								CommitMessage: "fix release validation",
							},
						},
					},
				},
			},
		},
	}
	task := &commonmodels.WorkflowTask{
		ProjectName: "demo",
		Stages: []*commonmodels.StageTask{
			{Jobs: []*commonmodels.JobTask{buildJob}},
		},
	}
	rulePlan := &commonmodels.AIReleaseSpecialistRulePlan{
		Contexts: []string{"runtime"},
	}

	input, err := BuildAIReleaseSpecialistInputFromTaskWithRulePlan(task, "ai-release-specialist", rulePlan)
	if err != nil {
		t.Fatalf("build input failed: %v", err)
	}
	if input.BuildSummary == nil {
		t.Fatal("expected build summary to remain available with a rule plan")
	}
	if len(input.BuildSummary.Items) != 1 {
		t.Fatalf("expected one build item, got %d", len(input.BuildSummary.Items))
	}
	if got, want := input.ChangeSummary.Branches, []string{"release/1.0"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("unexpected change branches, got %v want %v", got, want)
	}
	if got, want := input.ChangeSummary.CommitMessages, []string{"fix release validation"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("unexpected change commit messages, got %v want %v", got, want)
	}
}

func TestMergeReleaseTargetsClearsAmbiguousTopLevelEnvironment(t *testing.T) {
	targets := []*commonmodels.AIReleaseTargetsSummary{
		{
			EnvName:      "staging",
			EnvAlias:     "预发布",
			ServiceNames: []string{"service-a"},
			TargetCount:  1,
			Items: []*commonmodels.AIReleaseTargetItem{
				{
					JobName:      "deploy-staging",
					EnvName:      "staging",
					EnvAlias:     "预发布",
					ServiceNames: []string{"service-a"},
					TargetCount:  1,
				},
			},
		},
		{
			EnvName:      "prod",
			EnvAlias:     "生产",
			Production:   true,
			ServiceNames: []string{"service-b"},
			TargetCount:  1,
			Items: []*commonmodels.AIReleaseTargetItem{
				{
					JobName:      "deploy-prod",
					EnvName:      "prod",
					EnvAlias:     "生产",
					Production:   true,
					ServiceNames: []string{"service-b"},
					TargetCount:  1,
				},
			},
		},
	}

	merged := mergeReleaseTargets(targets)
	if merged.EnvName != "" || merged.EnvAlias != "" {
		t.Fatalf("expected ambiguous top-level environment to be empty, got %q/%q", merged.EnvName, merged.EnvAlias)
	}
	if merged.Production {
		t.Fatal("expected mixed production state not to be represented as production=true")
	}
	if got, want := merged.TargetCount, 2; got != want {
		t.Fatalf("unexpected merged target count, got %d want %d", got, want)
	}
	if len(merged.Items) != 2 {
		t.Fatalf("expected two detailed target items, got %d", len(merged.Items))
	}
	if got, want := merged.Items[1].EnvName, "prod"; got != want {
		t.Fatalf("expected detailed production target to remain intact, got %q", got)
	}
}

func TestMergeReleaseTargetsKeepsTopLevelEnvironmentForSingleEnvironment(t *testing.T) {
	merged := mergeReleaseTargets([]*commonmodels.AIReleaseTargetsSummary{
		{
			EnvName:      "prod",
			EnvAlias:     "生产",
			Production:   true,
			ServiceNames: []string{"service-a"},
			TargetCount:  1,
		},
		{
			EnvName:      "prod",
			EnvAlias:     "生产",
			Production:   true,
			ServiceNames: []string{"service-b"},
			TargetCount:  1,
		},
	})

	if got, want := merged.EnvName, "prod"; got != want {
		t.Fatalf("unexpected top-level environment, got %q want %q", got, want)
	}
	if got, want := merged.EnvAlias, "生产"; got != want {
		t.Fatalf("unexpected top-level environment alias, got %q want %q", got, want)
	}
	if !merged.Production {
		t.Fatal("expected single production environment to remain production=true")
	}
}

func TestFindAIRuntimeServiceUsesDeployJobType(t *testing.T) {
	helmService := &commonmodels.ProductService{
		ServiceName: "orders-release",
		Type:        setting.HelmDeployType,
	}
	chartService := &commonmodels.ProductService{
		ReleaseName: "orders-release",
		Type:        setting.HelmChartDeployType,
	}
	product := &commonmodels.Product{
		Services: [][]*commonmodels.ProductService{{
			helmService,
			chartService,
		}},
	}

	if got := findAIRuntimeService(product, "orders-release", string(config.JobZadigHelmChartDeploy)); got != chartService {
		t.Fatalf("expected helm chart service, got %#v", got)
	}
	if got := findAIRuntimeService(product, "orders-release", string(config.JobZadigHelmDeploy)); got != helmService {
		t.Fatalf("expected zadig helm service, got %#v", got)
	}
}

func TestResolveAIRuntimeServiceReleaseNameUsesServiceType(t *testing.T) {
	helmService := &commonmodels.ProductService{
		ServiceName: "orders",
		ReleaseName: "stale-release",
		Type:        setting.HelmDeployType,
	}
	releaseName, err := resolveAIRuntimeServiceReleaseName(
		"orders",
		helmService,
		map[string]string{"orders": "generated-orders-release"},
	)
	if err != nil {
		t.Fatalf("resolve zadig helm release name failed: %v", err)
	}
	if got, want := releaseName, "generated-orders-release"; got != want {
		t.Fatalf("unexpected zadig helm release name, got %q want %q", got, want)
	}

	chartService := &commonmodels.ProductService{
		ReleaseName: "stored-chart-release",
		Type:        setting.HelmChartDeployType,
	}
	releaseName, err = resolveAIRuntimeServiceReleaseName(
		"target-chart-release",
		chartService,
		nil,
	)
	if err != nil {
		t.Fatalf("resolve helm chart release name failed: %v", err)
	}
	if got, want := releaseName, "stored-chart-release"; got != want {
		t.Fatalf("unexpected helm chart release name, got %q want %q", got, want)
	}
}

func TestParseAIRuntimeWorkloadsFromHelmManifest(t *testing.T) {
	workloads, err := parseAIRuntimeWorkloadsFromHelmManifest(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: orders
spec:
  replicas: 3
  selector:
    matchLabels:
      app: orders
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: database
spec:
  replicas: 2
  selector:
    matchLabels:
      app: database
`)
	if err != nil {
		t.Fatalf("parse helm manifest failed: %v", err)
	}
	if len(workloads) != 2 {
		t.Fatalf("expected two workloads, got %d", len(workloads))
	}
	workloadsByName := make(map[string]*commonmodels.WorkLoad, len(workloads))
	for _, workload := range workloads {
		workloadsByName[workload.WorkloadName] = workload
	}
	if got := workloadsByName["orders"]; got == nil || got.WorkloadType != setting.Deployment || got.Replicas != 3 {
		t.Fatalf("unexpected orders workload: %#v", got)
	}
	if got := workloadsByName["database"]; got == nil || got.WorkloadType != setting.StatefulSet || got.Replicas != 2 {
		t.Fatalf("unexpected database workload: %#v", got)
	}
}

func TestGetAIRuntimeServiceWorkloadsLoadsHelmReleaseManifest(t *testing.T) {
	originalGetManifest := getAIReleaseHelmReleaseManifest
	t.Cleanup(func() {
		getAIReleaseHelmReleaseManifest = originalGetManifest
	})

	var requestedReleaseName string
	getAIReleaseHelmReleaseManifest = func(product *commonmodels.Product, releaseName string) (string, error) {
		requestedReleaseName = releaseName
		return `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: orders
spec:
  replicas: 1
`, nil
	}
	product := &commonmodels.Product{
		Services: [][]*commonmodels.ProductService{{
			{
				ServiceName: "orders",
				ReleaseName: "stale-release",
				Type:        setting.HelmDeployType,
			},
		}},
	}
	service := findAIRuntimeService(product, "orders", string(config.JobZadigHelmDeploy))
	target := &commonmodels.AIReleaseTargetsSummary{
		ServiceNames: []string{"orders"},
		Items: []*commonmodels.AIReleaseTargetItem{{
			JobType: string(config.JobZadigHelmDeploy),
		}},
	}
	if !needsAIRuntimeKubeClient(product, target) {
		t.Fatal("expected helm service to support runtime queries")
	}

	workloads, err := getAIRuntimeServiceWorkloads(product, service, "generated-orders-release")
	if err != nil {
		t.Fatalf("get helm runtime workloads failed: %v", err)
	}
	if got, want := requestedReleaseName, "generated-orders-release"; got != want {
		t.Fatalf("unexpected requested release, got %q want %q", got, want)
	}
	if len(workloads) != 1 || workloads[0].WorkloadName != "orders" {
		t.Fatalf("unexpected helm workloads: %#v", workloads)
	}
}

func TestGetAIRuntimeServiceWorkloadsReturnsK8SWorkloadsDirectly(t *testing.T) {
	want := []*commonmodels.WorkLoad{{
		WorkloadType: setting.Deployment,
		WorkloadName: "orders",
		Replicas:     2,
	}}
	service := &commonmodels.ProductService{
		Type:      setting.K8SDeployType,
		WorkLoads: want,
	}

	got, err := getAIRuntimeServiceWorkloads(&commonmodels.Product{}, service, "")
	if err != nil {
		t.Fatalf("get k8s runtime workloads failed: %v", err)
	}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("unexpected k8s workloads: %#v", got)
	}
}

func TestBuildAIRuntimeServiceItemUsesHelmChartReleaseName(t *testing.T) {
	item := buildAIRuntimeServiceItem(
		&commonmodels.Product{EnvName: "prod"},
		"orders-release",
		&commonmodels.ProductService{
			Type: setting.HelmChartDeployType,
		},
	)
	if got, want := item.ServiceName, "orders-release"; got != want {
		t.Fatalf("unexpected chart runtime service name, got %q want %q", got, want)
	}
}

func TestBuildAIReleaseSpecialistEvaluationPromptUsesCompactJSON(t *testing.T) {
	prompt, err := BuildAIReleaseSpecialistEvaluationPrompt(
		&commonmodels.AIReleaseSpecialistRulePlan{
			Contexts: []string{"runtime"},
			Rules: []*commonmodels.AIReleaseSpecialistRulePlanRule{{
				Dimension: "runtime",
			}},
		},
		"system",
		&commonmodels.AIReleaseSpecialistInput{
			ChangeSummary: &commonmodels.AIChangeSummary{Remark: "release"},
		},
	)
	if err != nil {
		t.Fatalf("build evaluation prompt failed: %v", err)
	}
	if strings.Contains(prompt, "评估规则计划：\n```json\n{\n  ") {
		t.Fatal("expected rule plan JSON to be compact")
	}
	if strings.Contains(prompt, "发布上下文:\n```json\n{\n  ") {
		t.Fatal("expected release context JSON to be compact")
	}
}

func TestBuildReleaseTargetFromHelmChartDeployKeepsProduction(t *testing.T) {
	job := &commonmodels.JobTask{
		OriginName: "chart-deploy",
		JobType:    string(config.JobZadigHelmChartDeploy),
		Spec: &commonmodels.JobTaskHelmChartDeploySpec{
			Env:        "prod",
			Production: true,
			DeployHelmChart: &commonmodels.DeployHelmChart{
				ReleaseName: "orders-release",
			},
		},
	}
	deploymentInfo, isDeployment, err := parseAIReleaseDeploymentJob(job)
	if err != nil || !isDeployment {
		t.Fatalf("parse helm chart deployment failed: isDeployment=%t err=%v", isDeployment, err)
	}
	target := deploymentInfo.buildReleaseTarget(job)
	if !target.Production {
		t.Fatal("expected helm chart target to be production")
	}
	if len(target.Items) != 1 || !target.Items[0].Production {
		t.Fatal("expected helm chart target item to be production")
	}
}
