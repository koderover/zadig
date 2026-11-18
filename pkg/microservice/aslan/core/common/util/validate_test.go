package util

import (
	"testing"

	"github.com/koderover/zadig/v2/pkg/shared/client/plutusenterprise"
)

func TestValidateZadigLicenseFeatureAI(t *testing.T) {
	for _, tc := range []struct {
		name     string
		edition  string
		status   string
		features []string
		want     bool
	}{
		{"basic with AI", plutusenterprise.ZadigSystemTypeBasic, plutusenterprise.ZadigXLicenseStatusNormal, []string{plutusenterprise.ZadigLicenseFeatureAI}, true},
		{"professional with AI", plutusenterprise.ZadigSystemTypeProfessional, plutusenterprise.ZadigXLicenseStatusNormal, []string{plutusenterprise.ZadigLicenseFeatureAI}, true},
		{"enterprise without AI", plutusenterprise.ZadigSystemTypeEnterprise, plutusenterprise.ZadigXLicenseStatusNormal, []string{plutusenterprise.ZadigLicenseFeatureDelivery}, false},
		{"expired with AI", plutusenterprise.ZadigSystemTypeEnterprise, "expired", []string{plutusenterprise.ZadigLicenseFeatureAI}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			license := &plutusenterprise.ZadigXLicenseStatus{Type: tc.edition, Status: tc.status, Features: tc.features}
			if got := validateZadigLicenseFeatureAI(license); got != tc.want {
				t.Fatalf("AI license valid = %t, want %t", got, tc.want)
			}
		})
	}
}
