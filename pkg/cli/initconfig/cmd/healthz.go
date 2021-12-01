package cmd

import (
	"net/http"
	"syscall"

	"github.com/koderover/zadig/pkg/shared/client/policy"
	"github.com/koderover/zadig/pkg/shared/client/user"
	"github.com/koderover/zadig/pkg/tool/log"
	"k8s.io/apiserver/pkg/server/healthz"
)

// InitHealthChecker
var InitHealthChecker healthz.HealthChecker = initCheck{}

type initCheck struct{}

func (initCheck) Name() string {
	return "initHealthChecker"
}

func (initCheck) Check(req *http.Request) error {
	if err := checkUserServiceHealth(); err != nil {
		log.Error("checkUserServiceHealth error:", err.Error())
		return err
	}
	if err := checkPolicyServiceHealth(); err != nil {
		log.Error("checkPolicyServiceHealth error:", err.Error())
		return err
	}

	once.Do(func() {
		go func() {
			defer syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
			err := Run()
			if err != nil {
				log.Errorf("once do init Run err:%s", err)
				return
			}
			log.Info("zadig init success")
		}()
	})
	return nil
}

func checkUserServiceHealth() error {
	return user.New().GetUserSvrHealthz()
}

func checkPolicyServiceHealth() error {
	return policy.NewDefault().GetPolicySvrHealthz()
}
