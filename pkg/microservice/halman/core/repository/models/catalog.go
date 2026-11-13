package models

type CatalogRequest struct {
	Keyword    string `json:"keyword,omitempty" jsonschema:"description=最多一百二十八字节；YAML 和构建模板填写用户指定的完整名称，按名称精确匹配；其他资源按原接口名称查询"`
	Page       int    `json:"page,omitempty" jsonschema:"description=YAML 和构建模板列表页码，从一开始，默认一；按返回的 next_page 继续查询，零表示结束"`
	Kind       string `json:"kind" jsonschema:"enum=projects,enum=codehosts,enum=repositories,enum=branches,enum=configuration_paths,enum=yaml_templates,enum=yaml_template,enum=helm_templates,enum=helm_template,enum=build_templates,enum=build_template,enum=clusters,enum=registries,enum=helm_repositories,enum=chart_versions,description=预置目录类型"`
	ProjectKey string `json:"project_key,omitempty" jsonschema:"description=项目标识；查询已有项目可用的 helm_repositories 时须提供"`
	ID         string `json:"id,omitempty" jsonschema:"description=查询结果中的模板标识；chart_versions 可填 Chart 仓库名称（repo_name），不是仓库内部 ID"`
	CodehostID int    `json:"codehost_id,omitempty" jsonschema:"description=已集成代码源标识"`
	RepoOwner  string `json:"repo_owner,omitempty" jsonschema:"description=仓库组织或拥有者"`
	RepoName   string `json:"repo_name,omitempty" jsonschema:"description=仓库名；chart_versions 必须提供 Chart 仓库名称（或通过 id 提供），返回该仓库全部 Chart 版本，一次查询可供多个服务复用"`
	Branch     string `json:"branch,omitempty" jsonschema:"description=分支名"`
	Path       string `json:"path,omitempty" jsonschema:"description=用户明确提供或原任务草稿已保存的服务配置文件或目录路径；未提供时补问，不猜测路径或搜索仓库，不读取整仓库"`
}
