`multi-service-demo` 目录结构说明：

``` bash
├── Dockerfile # Dockerfile，也可以作为 Dockerfile 模板，构建镜像时需要通过 --build-arg service=<值> 指定 service 参数的值
├── Makefile # 构建工程文件，构建时使用 make build-service* 即可
├── base-chart # Helm Chart 模板，可用于使用模板批量创建服务
├── full-charts # 多个 Helm Chart，service1、service2、service3 分别有完整独立的 Helm Chart 配置
├── general-chart # Helm Chart 模板，不可用于使用模板批量创建服务
├── k8s-yaml # K8s YAML 配置文件，用于 K8s YAML 项目
│   ├── service1 # service1 完整的配置
│   ├── service2 # service2 完整的配置
│   ├── service3 # service3 完整的配置
│   └── template.yaml # service1/service2/service3 的 K8s YAML 服务模板
├── src # 服务源代码
└── values # 使用 base-chart 模板批量创建服务时，多个服务的 values 文件
```
