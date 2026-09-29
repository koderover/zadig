package service

import (
	"reflect"
	"testing"

	"github.com/koderover/zadig/v2/pkg/setting"
)

func TestNormalizePrivateKeyProjects(t *testing.T) {
	tests := []struct {
		name     string
		projects []string
		want     []string
		wantErr  bool
	}{
		{name: "empty projects means all projects", want: []string{setting.AllProjects}},
		{name: "specific projects keep their scope", projects: []string{"project-a"}, want: []string{"project-a"}},
		{name: "all projects keeps the explicit scope", projects: []string{setting.AllProjects}, want: []string{setting.AllProjects}},
		{name: "all projects cannot be mixed", projects: []string{setting.AllProjects, "project-a"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePrivateKeyProjects(tt.projects)
			if (err != nil) != tt.wantErr {
				t.Fatalf("normalizePrivateKeyProjects() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("normalizePrivateKeyProjects() = %v, want %v", got, tt.want)
			}
		})
	}
}
