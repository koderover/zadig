package service

import (
	"strings"
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
)

func TestCleanAliyunRRSA(t *testing.T) {
	for _, tt := range []struct {
		name    string
		enabled bool
		roleARN string
		wantErr bool
	}{
		{name: "enabled", enabled: true, roleARN: "acs:ram::1234567890123456:role/zadig-build"},
		{name: "valid punctuation", enabled: true, roleARN: "acs:ram::1234567890123456:role/Zadig.Build-1"},
		{name: "at sign", enabled: true, roleARN: "acs:ram::1234567890123456:role/zadig@build", wantErr: true},
		{name: "underscore", enabled: true, roleARN: "acs:ram::1234567890123456:role/zadig_build", wantErr: true},
		{name: "maximum length", enabled: true, roleARN: "acs:ram::1234567890123456:role/" + strings.Repeat("a", 64)},
		{name: "too long", enabled: true, roleARN: "acs:ram::1234567890123456:role/" + strings.Repeat("a", 65), wantErr: true},
		{name: "missing role", enabled: true, wantErr: true},
		{name: "AWS role", enabled: true, roleARN: "arn:aws:iam::123456789012:role/zadig", wantErr: true},
		{name: "missing role name", enabled: true, roleARN: "acs:ram::1234567890123456:role/", wantErr: true},
		{name: "disabled"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cluster := &K8SCluster{
				Name: "ack", Provider: config.ClusterProviderAlibabaACK,
				AdvancedConfig: &AdvancedConfig{EnableIRSA: tt.enabled, IRSARoleARM: tt.roleARN},
			}
			if err := cluster.Clean(); (err != nil) != tt.wantErr {
				t.Fatalf("Clean() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
