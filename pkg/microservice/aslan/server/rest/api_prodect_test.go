package rest

import (
	"context"
	"net/http"
	"testing"

	"github.com/koderover/zadig/pkg/microservice/aslan/core"
	"github.com/steinfletcher/apitest"
)

//TestCreateProject (post) /api/project/products/:name  (get) /api/project/products/api-test-product
func TestCreateProject(t *testing.T){

	core.Start(context.Background())
	// 如果已经创建过了，跳过
	apitest.New().
		Observe(func(res *http.Response, req *http.Request, apiTest *apitest.APITest) {
			if res.StatusCode == 200{
				t.SkipNow()
			}
		}).
		Handler(NewEngine()).
		Get("/api/project/products/api-test-product").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
		Expect(t).
		End()

	// 创建项目
	apitest.New().
		Handler(NewEngine()).
		Post("/api/project/products").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
		Bodyf(`{"project_name":"api-test-product","product_name":"api-test-product","user_ids":[1],"team_id":null,"timeout":10,"desc":"xxxxxxx","visibility":"public","enabled":true,"product_feature":{"basic_facility":"kubernetes","deploy_type":"k8s"}}`).
		Expect(t).
		Status(http.StatusOK).
		Body(`{"message":"success"}`).
		End()

	// 查看是否存在
	apitest.New().
		Handler(NewEngine()).
		Get("/api/project/products/api-test-product").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
		Expect(t).
		Status(http.StatusOK).
		End()
}

//TestGetProjects 查询所有项目 (get) /api/project/products
func TestGetProjects(t *testing.T) {
	core.Start(context.Background())
	apitest.New().
		Handler(NewEngine()).
		Get("/api/project/products").Header("authorization", "X-ROOT-API-KEY 9F11B4E503C7F2B5").
		Expect(t).
		Status(http.StatusOK).
		End()
}
