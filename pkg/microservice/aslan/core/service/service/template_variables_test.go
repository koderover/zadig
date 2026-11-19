package service

import (
	"testing"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func TestNewServiceFromYamlTemplateInheritsCustomVariables(t *testing.T) {
	log.Init(&log.Config{Level: "error"})
	template := &commonmodels.YamlTemplate{
		Content:      "region: {{ .region }}\nenabled: {{ .enabled }}\n",
		VariableYaml: "region: west\nenabled: true\n",
		ServiceVariableKVs: []*commontypes.ServiceVariableKV{
			{Key: "region", Value: "west", Type: commontypes.ServiceVariableKVTypeEnum, Options: []string{"west", "east"}, Desc: "Deployment region"},
			{Key: "enabled", Value: true, Type: commontypes.ServiceVariableKVTypeBoolean},
		},
	}
	for _, tc := range []struct {
		name       string
		request    LoadServiceFromYamlTemplateReq
		enabled    bool
		wantRender string
	}{
		{name: "defaults", enabled: true, wantRender: "region: west\nenabled: true\n"},
		{
			name: "with override",
			request: LoadServiceFromYamlTemplateReq{
				VariableYaml:       "enabled: false\n",
				ServiceVariableKVs: []*commontypes.ServiceVariableKV{{Key: "enabled", Value: false, Type: commontypes.ServiceVariableKVTypeBoolean}},
			},
			wantRender: "region: west\nenabled: false\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.request.ProjectName, tc.request.ServiceName, tc.request.TemplateID = "demo", "api", "base"
			svc, err := newServiceFromYamlTemplate(template, &tc.request)
			if err != nil {
				t.Fatal(err)
			}
			kvs := make(map[string]*commontypes.ServiceVariableKV)
			for _, variable := range svc.ServiceVariableKVs {
				kvs[variable.Key] = variable
			}
			region, enabled := kvs["region"], kvs["enabled"]
			if len(svc.ServiceVariableKVs) != 2 || region == nil || enabled == nil || region.Value != "west" || enabled.Value != tc.enabled {
				t.Fatalf("unexpected persisted variables: %+v", svc.ServiceVariableKVs)
			}
			if region.Type != commontypes.ServiceVariableKVTypeEnum || len(region.Options) != 2 || region.Desc != "Deployment region" {
				t.Fatalf("inherited variable lost its metadata: %+v", region)
			}
			if svc.RenderedYaml != tc.wantRender {
				t.Fatalf("rendered YAML = %q, want %q", svc.RenderedYaml, tc.wantRender)
			}
		})
	}
}

func TestChartTemplateVariablesAreInheritedForRenderingAndStorage(t *testing.T) {
	defaults := []*commonmodels.ChartVariable{{Key: "region", Value: "west"}, {Key: "replicas", Value: "2"}}
	overrides := []*Variable{{Key: "replicas", Value: "3"}}
	variables := inheritChartTemplateVariables(overrides, defaults)
	got, err := renderVariablesToYaml("region: {{ .region }}\nreplicas: {{ .replicas }}\n", "demo", "api", variables)
	if err != nil || got != "region: west\nreplicas: 3\n" {
		t.Fatalf("rendered Chart values = %q, error %v", got, err)
	}
	creation := geneCreationDetail(&helmServiceCreationArgs{Source: setting.SourceFromChartTemplate, Variables: variables}).(*commonmodels.CreateFromChartTemplate)
	if len(creation.Variables) != 2 || creation.Variables[0].Value != "3" || creation.Variables[1].Value != "west" {
		t.Fatalf("unexpected persisted Chart variables: %+v", creation.Variables)
	}
}
