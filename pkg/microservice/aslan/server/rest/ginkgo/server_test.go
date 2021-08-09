package server_test

import (
	"github.com/koderover/zadig/pkg/microservice/aslan/core"
	"github.com/koderover/zadig/pkg/microservice/aslan/server/rest"
	"net/http"

	"context"
	. "github.com/onsi/ginkgo"
	"github.com/steinfletcher/apitest"
)

var _ = Describe("Ginkgo/Server", func() {

	var (
		t GinkgoTInterface
	)

	BeforeEach(func() {
		t = GinkgoT()
		core.Start(context.Background())
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
				Bodyf(`{"project_name":"api-test-product","product_name":"api-test-product","user_ids":[1],"team_id":null,"timeout":10,"desc":"xxxxxxx","visibility":"public","enabled":true,"product_feature":{"basic_facility":"kubernetes","deploy_type":"k8s"}}`).
				Expect(t).
				Status(http.StatusOK).
				Body(`{"message":"success"}`).
				End()

			// 创建服务
			apitest.New().
				Handler(rest.NewEngine()).
				Post("/api/service/services").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
				Body(`{"product_name":"api-test-product","service_name":"nginx","visibility":"private","type":"k8s","yaml":"---\napiVersion: v1\nkind: Service\nmetadata:\n  name: nginx\n  labels:\n    app: nginx\n    tier: backend\n    version: \"{{.nginxVersion}}\"\nspec:\n  type: NodePort\n  ports:\n  - port: 80\n  selector:\n    app: nginx\n    tier: backend\n---\napiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: nginx\nspec:\n  replicas: 2\n  selector:\n    matchLabels:\n      app: nginx\n      tier: backend\n      version: \"{{.nginxVersion}}\"\n  template:\n    metadata:\n      labels:\n        app: nginx\n        tier: backend\n        version: \"{{.nginxVersion}}\"\n    spec:\n      containers:\n      - name: nginx-test\n        image: ccr.ccs.tencentyun.com/shuhe/nginx:stable\n        ports:\n        - containerPort: 80\n        volumeMounts:\n          - name: static-page\n            mountPath: /usr/share/nginx/html\n      volumes:\n        - name: static-page\n          configMap:\n            name: static-page\n---\napiVersion: extensions/v1beta1\nkind: Ingress\nmetadata:\n  name: nginx-expose\nspec:\n  rules:\n  - host: $EnvName$-nginx-expose.app.8slan.com\n    http:\n      paths:\n      - backend:\n          serviceName: nginx\n          servicePort: 80\n        path: /\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: static-page\n  labels:\n    app.kubernetes.io/instance: poetry\n    app.kubernetes.io/name: poetry-portal-config\ndata:\n  index.html: |-\n        <u0021DOCTYPE html>\n        <html>\n        <head>\n            <meta charset=\"utf-8\" />\n            <meta http-equiv=\"X-UA-Compatible\" content=\"IE=edge\" />\n            <title>{{.customer}} - Sliding Perspective</title>\n            <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\" />\n            <style>\n            html,\n            body {\n                width: 100%;\n                height: 100%;\n                margin: 0;\n                background: #2d303a;\n                overflow-y: hidden;\n            }\n\n            .bounce {\n                display: flex;\n                align-items: center;\n                justify-content: center;\n                width: 100%;\n                color: white;\n                height: 100%;\n                font: normal bold 6rem 'Product Sans', sans-serif;\n                white-space: nowrap;\n            }\n\n            .letter {\n                animation: bounce 0.75s cubic-bezier(0.05, 0, 0.2, 1) infinite alternate;\n                display: inline-block;\n                transform: translate3d(0, 0, 0);\n                margin-top: 0.5em;\n                text-shadow: rgba(255, 255, 255, 0.4) 0 0 0.05em;\n                font: normal 500 6rem 'Varela Round', sans-serif;\n                color:#fff;\n                color:#1989fa;\n\n            }\n\n            .letter:nth-of-type(1) {\n                animation-delay: -0.083333333s;\n            }\n            .letter:nth-of-type(3) {\n                animation-delay: 0.0833333333s;\n            }\n            .letter:nth-of-type(4) {\n                animation-delay: 0.1666666667s;\n            }\n            .letter:nth-of-type(5) {\n                animation-delay: 0.25s;\n            }\n            .letter:nth-of-type(6) {\n                animation-delay: 0.3333333333s;\n            }\n            .letter:nth-of-type(7) {\n                animation-delay: 0.4166666667s;\n            }\n\n            @keyframes bounce {\n                0% {\n                transform: translate3d(0, 0, 0);\n                text-shadow: rgba(255, 255, 255, 0.4) 0 0 0.05em;\n                }\n                100% {\n                transform: translate3d(0, -1em, 0);\n                text-shadow: rgba(255, 255, 255, 0.4) 0 1em 0.35em;\n                }\n            }\n            </style>\n        </head>\n        <body>\n            <div class=\"bounce\">\n            <span class=\"letter\">K</span><span class=\"letter\"></span><span class=\"letter\">o</span><span class=\"letter\">d</span><span class=\"letter\">e</span><span class=\"letter\">R</span\n            ><span class=\"letter\">o</span><span class=\"letter\">v</span><span class=\"letter\">e</span><span class=\"letter\">r</span>\n            </div>\n        </body>\n        </html>","source":"spock"}`).
				Expect(t).
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
