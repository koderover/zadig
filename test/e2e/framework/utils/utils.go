package utils

import (
	"github.com/koderover/zadig/pkg/microservice/aslan/server/rest"
	"github.com/onsi/ginkgo"
	"github.com/steinfletcher/apitest"
	"io/ioutil"
	"net/http"
)

func QueryProduct(t ginkgo.GinkgoTInterface, by string) {
	ginkgo.By(by)
	apitest.New().
		//Observe(func(res *http.Response, req *http.Request, apiTest *apitest.APITest) {
		//	if res.StatusCode == http.StatusOK {
		//		t.Fail()
		//	}
		//}).
		Handler(rest.NewEngine("release")).
		Get("/api/project/products/api-test-product").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
		Expect(t).
		Status(http.StatusBadRequest).
		End()
}

func CreateProduct(t ginkgo.GinkgoTInterface, productName string, by string) {
	ginkgo.By(by)
	apitest.New().
		Handler(rest.NewEngine("release")).
		Post("/api/project/products").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
		Bodyf(`{"project_name":"%v","product_name":"%v","user_ids":[1],"team_id":null,"timeout":10,"desc":"","visibility":"public","enabled":true,"product_feature":{"basic_facility":"kubernetes","deploy_type":"k8s"}}`, productName, productName).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"message":"success"}`).
		End()
}

func CreateService(t ginkgo.GinkgoTInterface, by string) {
	ginkgo.By(by)
	apitest.New().
		Handler(rest.NewEngine("release")).
		Post("/api/service/services").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
		Body(`{"product_name":"api-test-product","service_name":"nginx","visibility":"private","type":"k8s","yaml":"---\napiVersion: v1\nkind: Service\nmetadata:\n  name: nginx\n  labels:\n    app: nginx\n    tier: backend\n    version: \"{{.nginxVersion}}\"\nspec:\n  type: NodePort\n  ports:\n  - port: 80\n  selector:\n    app: nginx\n    tier: backend\n---\napiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: nginx\nspec:\n  replicas: 2\n  selector:\n    matchLabels:\n      app: nginx\n      tier: backend\n      version: \"{{.nginxVersion}}\"\n  template:\n    metadata:\n      labels:\n        app: nginx\n        tier: backend\n        version: \"{{.nginxVersion}}\"\n    spec:\n      containers:\n      - name: nginx-test\n        image: ccr.ccs.tencentyun.com/shuhe/nginx:stable\n        ports:\n        - containerPort: 80\n        volumeMounts:\n          - name: static-page\n            mountPath: /usr/share/nginx/html\n      volumes:\n        - name: static-page\n          configMap:\n            name: static-page\n---\napiVersion: extensions/v1beta1\nkind: Ingress\nmetadata:\n  name: nginx-expose\nspec:\n  rules:\n  - host: $EnvName$-nginx-expose.app.8slan.com\n    http:\n      paths:\n      - backend:\n          serviceName: nginx\n          servicePort: 80\n        path: /\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: static-page\n  labels:\n    app.kubernetes.io/instance: poetry\n    app.kubernetes.io/name: poetry-portal-config\ndata:\n  index.html: |-\n        <u0021DOCTYPE html>\n        <html>\n        <head>\n            <meta charset=\"utf-8\" />\n            <meta http-equiv=\"X-UA-Compatible\" content=\"IE=edge\" />\n            <title>{{.customer}} - Sliding Perspective</title>\n            <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\" />\n            <style>\n            html,\n            body {\n                width: 100%;\n                height: 100%;\n                margin: 0;\n                background: #2d303a;\n                overflow-y: hidden;\n            }\n\n            .bounce {\n                display: flex;\n                align-items: center;\n                justify-content: center;\n                width: 100%;\n                color: white;\n                height: 100%;\n                font: normal bold 6rem 'Product Sans', sans-serif;\n                white-space: nowrap;\n            }\n\n            .letter {\n                animation: bounce 0.75s cubic-bezier(0.05, 0, 0.2, 1) infinite alternate;\n                display: inline-block;\n                transform: translate3d(0, 0, 0);\n                margin-top: 0.5em;\n                text-shadow: rgba(255, 255, 255, 0.4) 0 0 0.05em;\n                font: normal 500 6rem 'Varela Round', sans-serif;\n                color:#fff;\n                color:#1989fa;\n\n            }\n\n            .letter:nth-of-type(1) {\n                animation-delay: -0.083333333s;\n            }\n            .letter:nth-of-type(3) {\n                animation-delay: 0.0833333333s;\n            }\n            .letter:nth-of-type(4) {\n                animation-delay: 0.1666666667s;\n            }\n            .letter:nth-of-type(5) {\n                animation-delay: 0.25s;\n            }\n            .letter:nth-of-type(6) {\n                animation-delay: 0.3333333333s;\n            }\n            .letter:nth-of-type(7) {\n                animation-delay: 0.4166666667s;\n            }\n\n            @keyframes bounce {\n                0% {\n                transform: translate3d(0, 0, 0);\n                text-shadow: rgba(255, 255, 255, 0.4) 0 0 0.05em;\n                }\n                100% {\n                transform: translate3d(0, -1em, 0);\n                text-shadow: rgba(255, 255, 255, 0.4) 0 1em 0.35em;\n                }\n            }\n            </style>\n        </head>\n        <body>\n            <div class=\"bounce\">\n            <span class=\"letter\">K</span><span class=\"letter\"></span><span class=\"letter\">o</span><span class=\"letter\">d</span><span class=\"letter\">e</span><span class=\"letter\">R</span\n            ><span class=\"letter\">o</span><span class=\"letter\">v</span><span class=\"letter\">e</span><span class=\"letter\">r</span>\n            </div>\n        </body>\n        </html>","source":"spock"}`).
		Expect(t).
		Status(http.StatusOK).
		End()
}

func LoadServiceForVote(t ginkgo.GinkgoTInterface, by string) {
	ginkgo.By(by)
	apitest.New().
		Handler(rest.NewEngine("release")).
		Post("/api/service/loader/load/1/master").
		QueryParams(map[string]string{"repoOwner": "opensource", "repoName": "voting-app"}).
		Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
		Body(`{"product_name":"api-test-vote","visibility":"private","is_dir":true,"type":"k8s","path":"freestyle-k8s-specifications"}`).
		Expect(t).
		//Status(http.StatusOK).
		Body(`{"message":"success"}`).
		End()
}

func GetEnvInitInfo(t ginkgo.GinkgoTInterface, by string) (body []byte, err error) {
	ginkgo.By(by)
	apitest.New().
		Handler(rest.NewEngine("release")).
		Observe(func(res *http.Response, req *http.Request, apiTest *apitest.APITest) {
			body, err = ioutil.ReadAll(res.Body)
		}).
		Get("/api/environment/init_info/api-test-product").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
		Expect(t).
		End()
	return
}

func CreateBuild(t ginkgo.GinkgoTInterface, by string) {
	ginkgo.By(by)
	apitest.New().
		Handler(rest.NewEngine("release")).
		Post("/api/build/build").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
		Body(`{"timeout":60,"version":"stable","name":"test","desc":"","repos":[{"codehost_id":2,"repo_owner":"opensource","repo_name":"nginx","branch":"master","checkout_path":"","remote_name":"origin","submodules":false,"project_uuid":"","repo_uuid":"","repo_id":"","source":"gitlab"}],"pre_build":{"clean_workspace":false,"res_req":"low","build_os":"xenial","image_id":"610371c9dd57e20ea54bf1c0","image_from":"koderover","installs":[],"envs":[],"enable_proxy":false,"enable_gocov":false,"parameters":[]},"scripts":"#!/bin/bash\nset -e\n\ncd $WORKSPACE/zadig/examples/nginx\ndocker build -t $IMAGE -f Dockerfile .\ndocker push $IMAGE","main_file":"","post_build":{},"targets":[{"product_name":"api-test-product","service_name":"nginx","service_module":"nginx-test","key":"nginx/nginx-test"}],"product_name":"api-test-product","source":"zadig"}`).
		Expect(t).
		Status(http.StatusOK).
		End()
}

func CreateBuildVote(t ginkgo.GinkgoTInterface, by string) {
	ginkgo.By(by)
	apitest.New().
		Handler(rest.NewEngine("release")).
		Post("/api/build/build").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
		Body(`{"timeout":60,"version":"stable","name":"vote","desc":"","repos":[{"codehost_id":1,"repo_owner":"opensource","repo_name":"voting-app","branch":"master","checkout_path":"","remote_name":"origin","submodules":false,"project_uuid":"","repo_uuid":"","repo_id":"","source":"gitlab"}],"pre_build":{"clean_workspace":false,"res_req":"low","build_os":"xenial","image_id":"6112478453e7b7f840c38f90","image_from":"koderover","installs":[],"envs":[],"enable_proxy":false,"enable_gocov":false,"parameters":[]},"scripts":"#!/bin/bash\nset -e\n\ncd $WORKSPACE/voting-app/vote\ndocker build -t $IMAGE -f Dockerfile .\ndocker push $IMAGE","main_file":"","post_build":{},"targets":[{"product_name":"api-test-vote","service_name":"vote","service_module":"vote-e2e","key":"vote/vote-e2e"}],"product_name":"api-test-vote","source":"zadig"}`).
		Expect(t).
		Status(http.StatusOK).
		End()
	apitest.New().
		Handler(rest.NewEngine("release")).
		Post("/api/build/build").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
		Body(`{"timeout":60,"version":"stable","name":"result","desc":"","repos":[{"codehost_id":1,"repo_owner":"opensource","repo_name":"voting-app","branch":"master","checkout_path":"","remote_name":"origin","submodules":false,"project_uuid":"","repo_uuid":"","repo_id":"","source":"gitlab"}],"pre_build":{"clean_workspace":false,"res_req":"low","build_os":"xenial","image_id":"6112478453e7b7f840c38f90","image_from":"koderover","installs":[],"envs":[],"enable_proxy":false,"enable_gocov":false,"parameters":[]},"scripts":"#!/bin/bash\nset -e\n\ncd $WORKSPACE/voting-app/result\ndocker build -t $IMAGE -f Dockerfile .\ndocker push $IMAGE","main_file":"","post_build":{},"targets":[{"product_name":"api-test-vote","service_name":"result","service_module":"result-e2e","key":"result/result-e2e"}],"product_name":"api-test-vote","source":"zadig"}`).
		Expect(t).
		Status(http.StatusOK).
		End()
	apitest.New().
		Handler(rest.NewEngine("release")).
		Post("/api/build/build").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
		Body(`{"timeout":60,"version":"stable","name":"worker","desc":"","repos":[{"codehost_id":1,"repo_owner":"opensource","repo_name":"voting-app","branch":"master","checkout_path":"","remote_name":"origin","submodules":false,"project_uuid":"","repo_uuid":"","repo_id":"","source":"gitlab"}],"pre_build":{"clean_workspace":false,"res_req":"low","build_os":"xenial","image_id":"6112478453e7b7f840c38f90","image_from":"koderover","installs":[],"envs":[],"enable_proxy":false,"enable_gocov":false,"parameters":[]},"scripts":"#!/bin/bash\nset -e\n\ncd $WORKSPACE/voting-app/worker\ndocker build -t $IMAGE -f Dockerfile .\ndocker push $IMAGE","main_file":"","post_build":{},"targets":[{"product_name":"api-test-vote","service_name":"worker","service_module":"worker-e2e","key":"worker/worker-e2e"}],"product_name":"api-test-vote","source":"zadig"}`).
		Expect(t).
		Status(http.StatusOK).
		End()
}

func CreateAutoEnv(t ginkgo.GinkgoTInterface, by string) {
	ginkgo.By(by)
	apitest.New().
		Handler(rest.NewEngine("release")).
		Post("/api/environment/environments/api-test-vote/auto").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
		Query("envType", "k8s").
		Expect(t).
		Status(http.StatusOK).
		End()

	apitest.New().
		Handler(rest.NewEngine("release")).
		Post("/api/environment/environments/api-test-vote/auto").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
		Query("envType", "k8s").
		Expect(t).
		End()

	apitest.New().
		Handler(rest.NewEngine("release")).
		Post("/api/environment/environments/api-test-vote/auto").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
		Query("envType", "k8s").
		Expect(t).
		Status(http.StatusOK).
		End()
}

func CreateAutoWorkFlow(t ginkgo.GinkgoTInterface, by string) {
	ginkgo.By(by)
	apitest.New().
		Handler(rest.NewEngine("release")).
		Post("/api/workflow/workflow/api-test-vote/auto").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
		Query("envType", "k8s").
		Expect(t).
		Status(http.StatusOK).
		End()
}
