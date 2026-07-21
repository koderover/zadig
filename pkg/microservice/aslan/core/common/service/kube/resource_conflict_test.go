package kube

import (
	"strings"
	"testing"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestCheckResourceAppliedByOtherEnvs(t *testing.T) {
	serviceResource := &commonmodels.ServiceResource{
		GroupVersionKind: schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"},
		Name:             "service1",
	}
	clusterRoleResource := &commonmodels.ServiceResource{
		GroupVersionKind: schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"},
		Name:             "role1",
	}

	tests := []struct {
		name        string
		resources   []*commonmodels.ServiceResource
		envs        []*commonmodels.Product
		wantErrText string
	}{
		{
			name: "unmanaged resource is allowed",
		},
		{
			name: "resource managed by current service is allowed",
			envs: []*commonmodels.Product{{
				ProductName: "product1",
				EnvName:     "env1",
				Namespace:   "namespace1",
				Services: [][]*commonmodels.ProductService{{{
					ServiceName: "service1",
					Resources:   []*commonmodels.ServiceResource{serviceResource},
				}}},
			}},
		},
		{
			name: "resource managed by another service is rejected",
			envs: []*commonmodels.Product{{
				ProductName: "product1",
				EnvName:     "env1",
				Namespace:   "namespace1",
				Services: [][]*commonmodels.ProductService{{{
					ServiceName: "service2",
					Resources:   []*commonmodels.ServiceResource{serviceResource},
				}}},
			}},
			wantErrText: "resource is applied by other envs",
		},
		{
			name: "resource managed by another environment is rejected",
			envs: []*commonmodels.Product{{
				ProductName: "product2",
				EnvName:     "env2",
				Namespace:   "namespace1",
				Services: [][]*commonmodels.ProductService{{{
					ServiceName: "service2",
					Resources:   []*commonmodels.ServiceResource{serviceResource},
				}}},
			}},
			wantErrText: "resource is applied by other envs",
		},
		{
			name: "namespaced resource managed in another namespace is allowed",
			envs: []*commonmodels.Product{{
				ProductName: "product2",
				EnvName:     "env2",
				Namespace:   "namespace2",
				Services: [][]*commonmodels.ProductService{{{
					ServiceName: "service2",
					Resources:   []*commonmodels.ServiceResource{serviceResource},
				}}},
			}},
		},
		{
			name:      "cluster scoped resource managed in another namespace is rejected",
			resources: []*commonmodels.ServiceResource{clusterRoleResource},
			envs: []*commonmodels.Product{{
				ProductName: "product2",
				EnvName:     "env2",
				Namespace:   "namespace2",
				Services: [][]*commonmodels.ProductService{{{
					ServiceName: "service2",
					Resources:   []*commonmodels.ServiceResource{clusterRoleResource},
				}}},
			}},
			wantErrText: "resource is applied by other envs",
		},
	}

	productInfo := &commonmodels.Product{ProductName: "product1", EnvName: "env1", Namespace: "namespace1"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := tt.resources
			if resources == nil {
				resources = []*commonmodels.ServiceResource{serviceResource}
			}
			err := checkResourcesAppliedByOtherEnvs(resources, productInfo, "service1", tt.envs)
			if tt.wantErrText == "" {
				if err != nil {
					t.Fatalf("checkResourcesAppliedByOtherEnvs() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErrText) {
				t.Fatalf("checkResourcesAppliedByOtherEnvs() error = %v, want text %q", err, tt.wantErrText)
			}
		})
	}
}
