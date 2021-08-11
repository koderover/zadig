package server_test

import (
	"github.com/koderover/zadig/test/e2e/framework/utils"
	. "github.com/onsi/ginkgo"
	"time"
)

var _ = FDescribe("Demo/Vote", func() {
	Context("Pipline Create Voting Project", func() {
		It("create", func() {
			utils.CreateProduct(t, "api-test-vote", "create voting product")
			utils.LoadServiceForVote(t, "load service")
			utils.CreateBuildVote(t, "build")
			utils.CreateAutoEnv(t, "auto env")
			time.Sleep(time.Second * 8)
			utils.CreateAutoWorkFlow(t, "auto workflow")
		})
	})
})
