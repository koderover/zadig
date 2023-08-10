package migrate

import (
	"context"
	"testing"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
	"github.com/spf13/viper"
)

func Test_migrateClusterScheduleStrategy(t *testing.T) {
	log.Init(&log.Config{
		Level:    "debug",
		NoCaller: true,
	})
	mongotool.Init(context.TODO(), "mongodb://localhost:27019")
	viper.Set(setting.ENVAslanDBName, "zadig_ee_dev1")

	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name:    "test1",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := migrateClusterScheduleStrategy(); (err != nil) != tt.wantErr {
				t.Errorf("migrateClusterScheduleStrategy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
