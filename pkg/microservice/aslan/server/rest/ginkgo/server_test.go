package server_test

import (
	"encoding/json"
	"fmt"
	"github.com/koderover/zadig/pkg/microservice/aslan/core"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/server/rest"
	"github.com/koderover/zadig/pkg/tool/kube/client"
	"io/ioutil"
	"net/http"

	"context"
	. "github.com/onsi/ginkgo"
	"github.com/steinfletcher/apitest"
)

var _ = Describe("Ginkgo/Server", func() {

	var (
		t GinkgoTInterface
	)

	//BeforeEach(func() {
	//	t = GinkgoT()
	//	ctx := context.Background()
	//	core.Start(ctx)
	//})

	BeforeSuite(func() {
		t = GinkgoT()
		ctx := context.Background()
		core.Start(ctx)
		go client.Start(context.Background())
	})
	Context("Successful get all projects", func() {
		It("cookies should be set correctly", func() {
			apitest.New().
				Handler(rest.NewEngine()).
				Get("/api/project/products").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
				Expect(t).
				Status(http.StatusOK).
				End()
		})
	})

	Context("Successful CreateProject", func() {
		It("project should be create correctly if not have", func() {
			// 如果已经创建过了，跳过
			apitest.New().
				Observe(func(res *http.Response, req *http.Request, apiTest *apitest.APITest) {
					if res.StatusCode == 200 {
						t.SkipNow()
					}
				}).
				Handler(rest.NewEngine()).
				Get("/api/project/products/api-test-product").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
				Expect(t).
				End()

			// 创建项目
			apitest.New().
				Handler(rest.NewEngine()).
				Post("/api/project/products").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
				Bodyf(`{"project_name":"api-test-product","product_name":"api-test-product","user_ids":[1],"team_id":null,"timeout":10,"desc":"","visibility":"public","enabled":true,"product_feature":{"basic_facility":"kubernetes","deploy_type":"k8s"}}`).
				Expect(t).
				Status(http.StatusOK).
				Body(`{"message":"success"}`).
				End()

			// 创建服务
			By("create service")
			apitest.New().
				Handler(rest.NewEngine()).
				Post("/api/service/services").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
				Body(`{"product_name":"api-test-product","service_name":"nginx","visibility":"private","type":"k8s","yaml":"---\napiVersion: v1\nkind: Service\nmetadata:\n  name: nginx\n  labels:\n    app: nginx\n    tier: backend\n    version: \"{{.nginxVersion}}\"\nspec:\n  type: NodePort\n  ports:\n  - port: 80\n  selector:\n    app: nginx\n    tier: backend\n---\napiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: nginx\nspec:\n  replicas: 2\n  selector:\n    matchLabels:\n      app: nginx\n      tier: backend\n      version: \"{{.nginxVersion}}\"\n  template:\n    metadata:\n      labels:\n        app: nginx\n        tier: backend\n        version: \"{{.nginxVersion}}\"\n    spec:\n      containers:\n      - name: nginx-test\n        image: ccr.ccs.tencentyun.com/shuhe/nginx:stable\n        ports:\n        - containerPort: 80\n        volumeMounts:\n          - name: static-page\n            mountPath: /usr/share/nginx/html\n      volumes:\n        - name: static-page\n          configMap:\n            name: static-page\n---\napiVersion: extensions/v1beta1\nkind: Ingress\nmetadata:\n  name: nginx-expose\nspec:\n  rules:\n  - host: $EnvName$-nginx-expose.app.8slan.com\n    http:\n      paths:\n      - backend:\n          serviceName: nginx\n          servicePort: 80\n        path: /\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: static-page\n  labels:\n    app.kubernetes.io/instance: poetry\n    app.kubernetes.io/name: poetry-portal-config\ndata:\n  index.html: |-\n        <u0021DOCTYPE html>\n        <html>\n        <head>\n            <meta charset=\"utf-8\" />\n            <meta http-equiv=\"X-UA-Compatible\" content=\"IE=edge\" />\n            <title>{{.customer}} - Sliding Perspective</title>\n            <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\" />\n            <style>\n            html,\n            body {\n                width: 100%;\n                height: 100%;\n                margin: 0;\n                background: #2d303a;\n                overflow-y: hidden;\n            }\n\n            .bounce {\n                display: flex;\n                align-items: center;\n                justify-content: center;\n                width: 100%;\n                color: white;\n                height: 100%;\n                font: normal bold 6rem 'Product Sans', sans-serif;\n                white-space: nowrap;\n            }\n\n            .letter {\n                animation: bounce 0.75s cubic-bezier(0.05, 0, 0.2, 1) infinite alternate;\n                display: inline-block;\n                transform: translate3d(0, 0, 0);\n                margin-top: 0.5em;\n                text-shadow: rgba(255, 255, 255, 0.4) 0 0 0.05em;\n                font: normal 500 6rem 'Varela Round', sans-serif;\n                color:#fff;\n                color:#1989fa;\n\n            }\n\n            .letter:nth-of-type(1) {\n                animation-delay: -0.083333333s;\n            }\n            .letter:nth-of-type(3) {\n                animation-delay: 0.0833333333s;\n            }\n            .letter:nth-of-type(4) {\n                animation-delay: 0.1666666667s;\n            }\n            .letter:nth-of-type(5) {\n                animation-delay: 0.25s;\n            }\n            .letter:nth-of-type(6) {\n                animation-delay: 0.3333333333s;\n            }\n            .letter:nth-of-type(7) {\n                animation-delay: 0.4166666667s;\n            }\n\n            @keyframes bounce {\n                0% {\n                transform: translate3d(0, 0, 0);\n                text-shadow: rgba(255, 255, 255, 0.4) 0 0 0.05em;\n                }\n                100% {\n                transform: translate3d(0, -1em, 0);\n                text-shadow: rgba(255, 255, 255, 0.4) 0 1em 0.35em;\n                }\n            }\n            </style>\n        </head>\n        <body>\n            <div class=\"bounce\">\n            <span class=\"letter\">K</span><span class=\"letter\"></span><span class=\"letter\">o</span><span class=\"letter\">d</span><span class=\"letter\">e</span><span class=\"letter\">R</span\n            ><span class=\"letter\">o</span><span class=\"letter\">v</span><span class=\"letter\">e</span><span class=\"letter\">r</span>\n            </div>\n        </body>\n        </html>","source":"spock"}`).
				Expect(t).
				Status(http.StatusOK).
				End()

			// build
			By("create service build")
			apitest.New().
				Handler(rest.NewEngine()).
				Post("/api/build/build").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
				Body(`{"timeout":60,"version":"stable","name":"test","desc":"","repos":[{"codehost_id":2,"repo_owner":"opensource","repo_name":"nginx","branch":"master","checkout_path":"","remote_name":"origin","submodules":false,"project_uuid":"","repo_uuid":"","repo_id":"","source":"gitlab"}],"pre_build":{"clean_workspace":false,"res_req":"low","build_os":"xenial","image_id":"610371c9dd57e20ea54bf1c0","image_from":"koderover","installs":[],"envs":[],"enable_proxy":false,"enable_gocov":false,"parameters":[]},"scripts":"#!/bin/bash\nset -e\n\ncd $WORKSPACE/zadig/examples/nginx\ndocker build -t $IMAGE -f Dockerfile .\ndocker push $IMAGE","main_file":"","post_build":{},"targets":[{"product_name":"api-test-product","service_name":"nginx","service_module":"nginx-test","key":"nginx/nginx-test"}],"product_name":"api-test-product","source":"zadig"}`).
				Expect(t).
				Status(http.StatusOK).
				End()

			// 获取环境对应的service版本信息
			ret := &commonmodels.Product{}
			apitest.New().
				Handler(rest.NewEngine()).
				Observe(func(res *http.Response, req *http.Request, apiTest *apitest.APITest) {
					bs, _ := ioutil.ReadAll(res.Body)
					json.Unmarshal(bs, ret)
					fmt.Println(ret)
				}).
				Get("/api/environment/init_info/api-test-product").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
				Expect(t).
				End()

			// env
			By("create service env")
			apitest.New().
				Handler(rest.NewEngine()).
				Post("/api/environment/environments").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
				Bodyf(`{"product_name":"api-test-product","cluster_id":"","env_name":"test","source":"spock","vars":[{"key":"customer","value":"","alias":"{{.customer}}","state":"new","services":["nginx"]},{"key":"nginxVersion","value":"","alias":"{{.nginxVersion}}","state":"new","services":["nginx"]}],"revision":1,"isPublic":true,"roleIds":[],"services":[[{"service_name":"nginx","type":"k8s","revision":%d,"containers":[{"name":"nginx-test","image":"ccr.ccs.tencentyun.com/trial/nginx-test:20210803104451-1-release-1.3.0"}],"picked":true}]]}`, ret.Services[0][0].Revision).
				Expect(t).
				Body(`{"message":"success"}`).
				Status(http.StatusOK).
				End()

			// 查看是否存在
			apitest.New().
				Handler(rest.NewEngine()).
				Get("/api/project/products/api-test-product").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
				Expect(t).
				Status(http.StatusOK).
				End()
		})
	})
})
