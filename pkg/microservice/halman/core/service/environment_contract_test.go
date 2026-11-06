package service

import (
	"context"
	"net/http"
	"testing"

	envservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func TestEnvironmentIdentityUsesAslanResponseContract(t *testing.T) {
	for _, production := range []bool{false, true} {
		platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
			w.Write(common.JSON(envservice.ProductResp{ProductName: "p", EnvName: "dev", Production: production, IsProd: !production}))
		})
		_, err := platform.environment(context.Background(), token("user"), models.ServiceTarget{ProjectKey: "p", EnvName: "dev", ServiceName: "api", Production: production})
		if err != nil {
			t.Fatalf("原接口的实际响应必须兼容，环境属性不能与集群属性混淆: production=%v %v", production, err)
		}
	}
}
