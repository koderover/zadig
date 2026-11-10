package kube

import (
	"bytes"
	"io"
	"reflect"
	"testing"
	"text/template"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	corev1 "k8s.io/api/core/v1"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
)

func TestWorkflowServiceAccountTemplates(t *testing.T) {
	for _, tt := range []struct {
		name     string
		provider int8
		enabled  bool
		roleARN  string
		want     map[string]string
	}{
		{name: "AWS", provider: config.ClusterProviderAmazonEKS, enabled: true, roleARN: "arn:aws:iam::123456789012:role/zadig", want: map[string]string{"eks.amazonaws.com/role-arn": "arn:aws:iam::123456789012:role/zadig"}},
		{name: "Alibaba", provider: config.ClusterProviderAlibabaACK, enabled: true, roleARN: "acs:ram::1234567890123456:role/zadig-build", want: map[string]string{"pod-identity.alibabacloud.com/role-name": "zadig-build"}},
		{name: "disabled", provider: config.ClusterProviderAlibabaACK},
	} {
		for name, tmpl := range map[string]*template.Template{
			"attached":  template.Must(template.New("workflow").Parse(WorkflowResourceYaml)),
			"namespace": YamlTemplateForNamespace,
		} {
			t.Run(tt.name+"/"+name, func(t *testing.T) {
				annotations := WorkflowServiceAccountAnnotations(tt.provider, tt.enabled, tt.roleARN)
				if !reflect.DeepEqual(annotations, tt.want) {
					t.Fatalf("annotations = %v, want %v", annotations, tt.want)
				}
				var rendered bytes.Buffer
				if err := tmpl.Execute(&rendered, TemplateSchema{Namespace: "custom-agent", IRSAAnnotations: annotations}); err != nil {
					t.Fatal(err)
				}
				decoder := yamlutil.NewYAMLOrJSONDecoder(&rendered, 4096)
				for {
					var sa corev1.ServiceAccount
					if err := decoder.Decode(&sa); err == io.EOF {
						t.Fatal("workflow ServiceAccount not found")
					} else if err != nil {
						t.Fatal(err)
					}
					if sa.Kind != "ServiceAccount" || sa.Name != "workflow-cm-sa" {
						continue
					}
					if !reflect.DeepEqual(sa.Annotations, tt.want) {
						t.Fatalf("rendered annotations = %v, want %v", sa.Annotations, tt.want)
					}
					break
				}
			})
		}
	}
}
