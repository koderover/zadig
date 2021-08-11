package server_test

import (
	"github.com/koderover/zadig/pkg/microservice/aslan/core"
	"github.com/koderover/zadig/pkg/tool/kube/client"
	"testing"

	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var t GinkgoTInterface

func TestHandlers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Server Suite")
}

var _ = Describe("init client", func() {
	BeforeSuite(func() {
		t = GinkgoT()
		ctx := context.Background()
		core.Start(ctx)
		go client.Start(context.Background())
	})
})
