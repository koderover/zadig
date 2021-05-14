/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package errors

var (
	//-----------------------------------------------------------------------------------------------
	// Standard Error
	//-----------------------------------------------------------------------------------------------

	// ErrInvalidParam ...
	ErrInvalidParam = NewHTTPError(400, "Bad Request")
	// ErrUnauthorized ...
	ErrUnauthorized = NewHTTPError(401, "Unauthorized")
	// ErrForbidden ...
	ErrForbidden = NewHTTPError(403, "Forbidden")
	// ErrNotFound ...
	ErrNotFound = NewHTTPError(404, "Request Not Found")
	// ErrInternalError ...
	ErrInternalError = NewHTTPError(500, "Internal Error")

	//-----------------------------------------------------------------------------------------------
	// User APIs Range: 6000 - 6019
	//-----------------------------------------------------------------------------------------------

	// ErrCreateUser ...
	ErrCreateUser = NewHTTPError(6000, "创建用户信息失败")
	// ErrUpdateUser ...
	ErrUpdateUser = NewHTTPError(6001, "更新用户信息失败")
	// ErrListUsers ...
	ErrListUsers = NewHTTPError(6002, "列出用户信息失败")
	// ErrFindUser ...
	ErrFindUser = NewHTTPError(6002, "获取用户信息失败")

	//-----------------------------------------------------------------------------------------------
	// Team APIs Range: 6020 - 6039
	//-----------------------------------------------------------------------------------------------

	// ErrCreateTeam ...
	ErrCreateTeam = NewHTTPError(6021, "创建团队信息失败")
	// ErrGetTeam ...
	ErrGetTeam = NewHTTPError(6022, "根据ID获取团队信息失败")
	// ErrListTeams ...
	ErrListTeams = NewHTTPError(6023, "列出团队信息失败")
	// ErrUpdateTeam ...
	ErrUpdateTeam = NewHTTPError(6024, "更新团队信息失败")
	// ErrDeleteTeam ...
	ErrDeleteTeam = NewHTTPError(6025, "删除团队信息失败")
	// ErrFindUserTeams ...
	ErrFindUserTeams = NewHTTPError(6026, "获取用户团队信息失败")
	// ErrCreateProductTeam
	ErrCreateProductTeam = NewHTTPError(6027, "创建项目团队信息失败")
	// ErrDeleteProductTeam
	ErrDeleteProductTeam = NewHTTPError(6028, "删除项目团队信息失败")

	//-----------------------------------------------------------------------------------------------
	// Template APIs Range: 6040 - 6059
	//-----------------------------------------------------------------------------------------------

	// ErrCreateTemplate ...
	ErrCreateTemplate = NewHTTPError(6040, "创建模板失败")
	// ErrUpdateTemplate ...
	ErrUpdateTemplate = NewHTTPError(6041, "更新模板失败")
	// ErrListTemplate ...
	ErrListTemplate = NewHTTPError(6042, "列出模板失败")
	// ErrGetTemplate ...
	ErrGetTemplate = NewHTTPError(6043, "获取模板失败")
	// ErrDeleteTemplate ...
	ErrDeleteTemplate = NewHTTPError(6044, "删除模板失败")
	// ErrValidateTemplate ...
	ErrValidateTemplate = NewHTTPError(6045, "验证模板失败")
	// ErrCountTemplate ...
	ErrCountTemplate = NewHTTPError(6046, "模板计数失败")
	// ErrGetRenderSetKeys ...
	ErrGetRenderSetKeys = NewHTTPError(6047, "获取渲染配置键失败")
	// ErrCreateRenderSet ...
	ErrCreateRenderSet = NewHTTPError(6048, "创建渲染配置集失败")
	// ErrListRenderSets ...
	ErrListRenderSets = NewHTTPError(6049, "列出渲染配置集失败")
	// ErrDeleteRenderSet ...
	ErrDeleteRenderSet = NewHTTPError(6050, "删除渲染配置集失败")
	// ErrSetDefaultRenderSet ...
	ErrSetDefaultRenderSet = NewHTTPError(6051, "设置默认渲染配置集失败")
	// ErrGetRenderSet ...
	ErrGetRenderSet = NewHTTPError(6052, "获取渲染配置集失败")
	// ErrUpdateRenderSet ...
	ErrUpdateRenderSet = NewHTTPError(6053, "更新渲染配置集失败")
	// ErrPreloadServiceTemplate
	ErrPreloadServiceTemplate = NewHTTPError(6054, "从代码库获取服务列表失败")
	// ErrLoadServiceTemplate
	ErrLoadServiceTemplate = NewHTTPError(6055, "从代码库导入服务失败")
	// ErrUpdateServiceGroupTemplate
	ErrUpdateServiceGroupTemplate = NewHTTPError(6056, "更新服务组失败")
	// ErrValidateServiceUpdate
	ErrValidateServiceUpdate = NewHTTPError(6057, "更新服务配置失败")

	//-----------------------------------------------------------------------------------------------
	// Product APIs Range: 6060 - 6079
	//-----------------------------------------------------------------------------------------------

	// ErrCreateProduct ...
	ErrCreateProduct = NewHTTPError(6060, "创建产品失败")
	// ErrListProducts ...
	ErrListProducts = NewHTTPError(6061, "列出产品失败")
	// ErrUpdateProduct ...
	ErrUpdateProduct = NewHTTPError(6062, "更新产品失败")
	// ErrDeleteProduct ...
	ErrDeleteProduct = NewHTTPError(6063, "删除产品失败")
	// ErrDeleteProducts ...
	ErrDeleteProductTempl = NewHTTPError(6079, "项目删除检查失败，因为存在正在使用的环境!")
	// ErrGetProduct ...
	ErrGetProduct = NewHTTPError(6073, "获取产品失败")
	// ErrListActiveProducts ...
	ErrListActiveProducts = NewHTTPError(6064, "列出创建,更新,删除中产品失败")
	// ErrListGroups ...
	ErrListGroups = NewHTTPError(6065, "列出服务组失败")
	// ErrListProductsRevision ...
	ErrListProductsRevision = NewHTTPError(6066, "列出产品版本失败")
	// ErrGetProductRevision ...
	ErrGetProductRevision = NewHTTPError(6067, "获取产品版本失败")
	// ErrFindProduct ...
	ErrFindProduct = NewHTTPError(6068, "获取集成环境失败")
	// ErrGetProductAuth ...
	ErrGetProductAuth = NewHTTPError(6069, "获取产品权限失败")
	// ErrUpdateProductAuth ...
	ErrUpdateProductAuth = NewHTTPError(6070, "更新产品权限失败")
	// ErrPatchProduct ...
	ErrPatchProduct = NewHTTPError(6071, "产品支持集成测试覆盖率失败")
	// ErrStopPatchProduct ...
	ErrStopPatchProduct = NewHTTPError(6072, "产品收集集成测试覆盖率失败")
	// ErrCreateEnv ...
	ErrCreateEnv = NewHTTPError(6074, "创建集成环境失败")
	// ErrListEnvs ...
	ErrListEnvs = NewHTTPError(6075, "列出集成环境失败")
	// ErrUpdateEnv ...
	ErrUpdateEnv = NewHTTPError(6076, "更新集成环境失败")
	// ErrDeleteEnv ...
	ErrDeleteEnv = NewHTTPError(6077, "删除集成环境失败")
	// ErrGetEnv ...
	ErrGetEnv = NewHTTPError(6078, "获取集成环境失败")
	// ErrFindProductTmpl ...
	ErrFindProductTmpl = NewHTTPError(6079, "项目已删除，环境正在回收中")
	// TODO: max error code reached, sharing error code with create product
	ErrForkProduct = NewHTTPError(6060, "Fork开源项目失败")
	// TODO: max error code reached, sharing error code with delete product
	ErrUnForkProduct = NewHTTPError(6063, "删除Fork环境失败")

	//-----------------------------------------------------------------------------------------------
	// Product Service APIs Range: 6080 - 6099
	//-----------------------------------------------------------------------------------------------

	// ErrRestartService ...
	ErrRestartService = NewHTTPError(6080, "重启服务失败")
	// ErrScaleService ...
	ErrScaleService = NewHTTPError(6081, "伸缩服务失败")
	// ErrUpdateConainterImage ...
	ErrUpdateConainterImage = NewHTTPError(6082, "更新服务镜像失败")
	// ErrGetService ...
	ErrGetService = NewHTTPError(6083, "获取服务失败")
	// ErrGetServiceContainer ...
	ErrGetServiceContainer = NewHTTPError(6084, "获取服务容器失败")
	// ErrListServicePod ...
	ErrListServicePod = NewHTTPError(6085, "列出服务Pod失败")
	// ErrDeletePod ...
	ErrDeletePod = NewHTTPError(6086, "删除服务Pod失败")
	// ErrGetConfigMap ...
	ErrGetConfigMap = NewHTTPError(6087, "获取服务配置失败")
	// ErrListConfigMaps ...
	ErrListConfigMaps = NewHTTPError(6088, "列出服务配置失败")
	// ErrUpdateConfigMap ...
	ErrUpdateConfigMap = NewHTTPError(6089, "更新服务配置失败")
	// ErrCreateConfigMap ...
	ErrCreateConfigMap = NewHTTPError(6090, "创建服务配置失败")
	// ErrRollBackConfigMap ...
	ErrRollBackConfigMap = NewHTTPError(6091, "回滚服务配置失败")
	// ErrUpdateService ...
	ErrUpdateService = NewHTTPError(6092, "更新服务失败")
	// ErrListPodEvents ...
	ErrListPodEvents = NewHTTPError(6093, "列出服务事件失败")

	//-----------------------------------------------------------------------------------------------
	// it report APIs Range: 6100 - 6199
	//-----------------------------------------------------------------------------------------------

	// ErrGetItReport ...
	ErrGetItReport = NewHTTPError(6100, "获取测试报告失败")
	// ErrUpsertItReport ...
	ErrUpsertItReport = NewHTTPError(6101, "更新测试报告失败")

	//-----------------------------------------------------------------------------------------------
	// install APIs Range: 6120 - 6139
	//-----------------------------------------------------------------------------------------------

	// ErrGetInstall ...
	ErrGetInstall = NewHTTPError(6120, "获取安装脚本失败")
	// ErrCreateInstall ...
	ErrCreateInstall = NewHTTPError(6121, "创建安装脚本失败")
	// ErrUpdateInstall ...
	ErrUpdateInstall = NewHTTPError(6122, "更新安装脚本失败")
	// ErrListInstalls ...
	ErrListInstalls = NewHTTPError(6123, "列出安装脚本失败")
	// ErrDeleteInstall ...
	ErrDeleteInstall = NewHTTPError(6124, "删除安装脚本失败")

	//-----------------------------------------------------------------------------------------------
	// Pipeline APIs Range: 6140 - 6159
	//-----------------------------------------------------------------------------------------------

	// ErrCreatePipeline ...
	ErrCreatePipeline = NewHTTPError(6140, "创建工作流失败")
	// ErrUpdatePipeline ...
	ErrUpdatePipeline = NewHTTPError(6141, "更新工作流失败")
	// ErrListPipeline ...
	ErrListPipeline = NewHTTPError(6142, "列出工作流失败")
	// ErrGetPipeline ...
	ErrGetPipeline = NewHTTPError(6143, "获取工作流失败")
	// ErrDeletePipeline ...
	ErrDeletePipeline = NewHTTPError(6144, "删除工作流失败")
	// ErrExistsPipeline ...
	ErrExistsPipeline = NewHTTPError(6145, "工作流已经存在")
	// ErrCleanWorkspace ...
	ErrCleanWorkspace = NewHTTPError(6146, "清理工作目录失败")
	// ErrListWorkspace ...
	ErrListWorkspace = NewHTTPError(6147, "列出工作目录失败")
	// ErrGetWorkspaceFile ...
	ErrGetWorkspaceFile = NewHTTPError(6147, "获取工作目录文件失败")
	// ErrRenamePipeline ...
	ErrRenamePipeline = NewHTTPError(6148, "更新工作流名称失败")
	// ErrListFavorite
	ErrListFavorite = NewHTTPError(6149, "列出收藏失败")
	// ErrFilePath
	ErrFilePath = NewHTTPError(6150, "获取文件目录失败")
	// ErrFileContent
	ErrFileContent = NewHTTPError(6151, "获取文件内容失败")
	// ErrListRepoDir
	ErrListRepoDir = NewHTTPError(6152, "列出repo目录失败")

	//-----------------------------------------------------------------------------------------------
	// Pipeline Task APIs Range: 6160 - 6179
	//-----------------------------------------------------------------------------------------------

	// ErrCreateTask ...
	ErrCreateTask = NewHTTPError(6160, "创建工作流任务失败")
	// ErrGetTask ...
	ErrGetTask = NewHTTPError(6161, "获取工作流任务失败")
	// ErrListTasks ...
	ErrListTasks = NewHTTPError(6162, "列出工作流任务失败")
	// ErrCancelTask ...
	ErrCancelTask = NewHTTPError(6163, "取消工作流任务失败")
	// ErrRestartTask ...
	ErrRestartTask = NewHTTPError(6164, "重试工作流任务失败")
	// ErrListPipelinesTaskStatus ...
	ErrListPipelinesTaskStatus = NewHTTPError(6165, "列出工作流任务状态失败")
	// ErrCreateGithubTask ...
	ErrCreateGithubTask = NewHTTPError(6166, "创建Git工作流任务失败")
	// ErrCountTasks ...
	ErrCountTasks = NewHTTPError(6167, "工作流计数失败")

	ErrCreateTaskFailed = NewHTTPError(6168, "创建工作流任务失败")

	//-----------------------------------------------------------------------------------------------
	// Keystore APIs Range: 6180 - 6189
	//-----------------------------------------------------------------------------------------------

	// ErrUpsertKeyStore ...
	ErrUpsertKeyStore = NewHTTPError(6180, "更新敏感信息失败")
	// ErrListKeyStores ...
	ErrListKeyStores = NewHTTPError(6181, "列出敏感信息失败")

	//-----------------------------------------------------------------------------------------------
	// Counter APIs Range: 6190 - 6199
	//-----------------------------------------------------------------------------------------------

	// ErrGetCounter ...
	ErrGetCounter = NewHTTPError(6190, "获取计数器失败")
	// ErrCreateCounter ...
	ErrCreateCounter = NewHTTPError(6191, "创建计数器失败")
	// ErrUpdateCounter ...
	ErrUpdateCounter = NewHTTPError(6192, "更新计数器失败")
	// ErrDeleteCounter ...
	ErrDeleteCounter = NewHTTPError(6193, "删除计数器失败")

	//-----------------------------------------------------------------------------------------------
	// Github APIs Range: 6200 - 6219
	//-----------------------------------------------------------------------------------------------

	// ErrGithubListRepos ...
	ErrGithubListRepos = NewHTTPError(6200, "列出Git仓库失败")
	// ErrGithubListBranches ...
	ErrGithubListBranches = NewHTTPError(6201, "列出Git仓库分支失败")
	// ErrGithubListPullRequests ...
	ErrGithubListPullRequests = NewHTTPError(6202, "列出Git仓库PR失败")
	// ErrGithubGetPullRequest ...
	ErrGithubGetPullRequest = NewHTTPError(6203, "获取仓库PR失败")
	// ErrGithubListCommits ...
	ErrGithubListCommits = NewHTTPError(6204, "列出仓库Commit失败")
	// ErrGithubCreateHook ...
	ErrGithubCreateHook = NewHTTPError(6205, "创建仓库WebHook失败")
	// ErrGithubQueryPullRequestsWithCommits ...
	ErrGithubQueryPullRequestsWithCommits = NewHTTPError(6206, "根据起始结束Commit列出Git仓库PR失败")
	// ErrGithubWebHook ...
	ErrGithubWebHook = NewHTTPError(6207, "trigger pipeline error")
	// ErrGithubListTags ...
	ErrGithubListTags = NewHTTPError(6208, "列出 Git Tags 失败")
	// ErrGithubListReleases ...
	ErrGithubListReleases = NewHTTPError(6209, "列出 Git Releases 失败")
	// ErrGithubUpdateStatus ...
	ErrGithubUpdateStatus = NewHTTPError(6210, "更新 Git Status 失败")
	// ErrGithubListInfos ...
	ErrGithubListInfos = NewHTTPError(6211, "列出 Git 信息失败")

	//-----------------------------------------------------------------------------------------------
	// Notify APIs Range: 6220 - 6239
	//-----------------------------------------------------------------------------------------------

	// ErrCreateNotify ...
	ErrCreateNotify = NewHTTPError(6220, "创建消息失败")
	// ErrUpdateNotify ...
	ErrUpdateNotify = NewHTTPError(6229, "更新消息失败")
	// ErrDeleteNotifies ...
	ErrDeleteNotifies = NewHTTPError(6227, "删除消息失败")
	// ErrReadNotify ...
	ErrReadNotify = NewHTTPError(6226, "设置已读消息失败")
	// ErrPullNotifyAnnouncement ...
	ErrPullNotifyAnnouncement = NewHTTPError(6221, "获取公告失败")
	// ErrPullAllAnnouncement ...
	ErrPullAllAnnouncement = NewHTTPError(6222, "获取公告失败")
	// ErrPullNotify ..
	ErrPullNotify = NewHTTPError(6223, "获取消息失败")

	// ErrSubscribeNotify ...
	ErrSubscribeNotify = NewHTTPError(6224, "订阅消息失败")
	// ErrUnsubscribeNotify ...
	ErrUnsubscribeNotify = NewHTTPError(6225, "取消订阅消息失败")
	// ErrListSubscriptions ...
	ErrListSubscriptions = NewHTTPError(6228, "列订阅消息失败")
	// ErrUpdateSubscribe ...
	ErrUpdateSubscribe = NewHTTPError(6230, "更新订阅失败")

	//-----------------------------------------------------------------------------------------------
	// Logs APIs Range: 6260 - 6279
	//-----------------------------------------------------------------------------------------------

	// ErrQueryContainerLogs ...
	ErrQueryContainerLogs = NewHTTPError(6260, "查询容器日志失败")
	// ErrBuildJobContainerLogs ...
	ErrBuildJobContainerLogs = NewHTTPError(6261, "查询编译容器日志失败")
	// ErrTestJobContainerLogs ...
	ErrTestJobContainerLogs = NewHTTPError(6262, "查询测试容器日志失败")

	//-----------------------------------------------------------------------------------------------
	// Registry APIs Range: 6280 - 6299
	//-----------------------------------------------------------------------------------------------

	// ErrListImages ...
	ErrListImages   = NewHTTPError(6280, "列出镜像失败")
	ErrFindRegistry = NewHTTPError(6281, "找不到指定的镜像仓库")

	//-----------------------------------------------------------------------------------------------
	// Insghts APIs Range: 6300 - 6399
	//-----------------------------------------------------------------------------------------------

	// ErrGetTeamInsights ...
	ErrGetTeamInsights = NewHTTPError(6300, "获取团队统计信息失败")
	// ErrGetProductInsights ...
	ErrGetProductInsights = NewHTTPError(6301, "获取产品统计信息失败")
	// ErrGetPipelineInsights ...
	ErrGetPipelineInsights = NewHTTPError(6302, "获取工作流统计信息失败")
	// ErrCreateInsightsConfig ...
	ErrCreateInsightsConfig = NewHTTPError(6303, "创建仪表盘配置失败")
	// ErrListInsightsConfig ...
	ErrListInsightsConfig = NewHTTPError(6304, "列出仪表盘配置失败")
	// ErrUpdateInsightsConfig ...
	ErrUpdateInsightsConfig = NewHTTPError(6305, "更新仪表盘配置失败")
	// ErrDeleteInsightsConfig ...
	ErrDeleteInsightsConfig = NewHTTPError(6306, "删除仪表盘配置失败")
	// ErrGetProjectInsights ...
	ErrGetProjectInsights = NewHTTPError(6307, "获取项目进度信息失败")
	// ErrGetQualityInsights ...
	ErrGetQualityInsights = NewHTTPError(6308, "获取质量现状信息失败")

	//-----------------------------------------------------------------------------------------------
	// Kube APIs Range: 6400 - 6499
	//-----------------------------------------------------------------------------------------------

	// ErrCreateNamspace ...
	ErrCreateNamspace = NewHTTPError(6400, "创建用户namespace失败")
	// ErrCreateSecret ...
	ErrCreateSecret = NewHTTPError(6401, "创建secret失败")
	// ErrUpdateSecret ...
	ErrUpdateSecret = NewHTTPError(6402, "更新secret失败")

	//-----------------------------------------------------------------------------------------------
	// Gitlab APIs Range: 6500 - 6519
	//-----------------------------------------------------------------------------------------------

	// ErrGitlabListProjects ...
	ErrGitlabListProjects = NewHTTPError(6500, "列出Gitlab仓库失败")
	// ErrGitlabListGroupProjects ...
	ErrGitlabListGroupProjects = NewHTTPError(6504, "列出Gitlab仓库失败2")
	// ErrGitlabGetProject ...
	ErrGitlabGetProject = NewHTTPError(6501, "查询Gitlab仓库失败")
	// ErrGitlabListBranches ...
	ErrGitlabListBranches = NewHTTPError(6502, "列出Gitlab仓库分支失败")
	// ErrGitlabListMergsRequests ...
	ErrGitlabListMergsRequests = NewHTTPError(6503, "列出Gitlab仓库MR失败")

	//-----------------------------------------------------------------------------------------------
	// Module APIs Range: 6520 - 6539
	//-----------------------------------------------------------------------------------------------

	// ErrCreateBuildModule ...
	ErrCreateBuildModule = NewHTTPError(6520, "新建编译模块失败")
	// ErrUpdateBuildModule ...
	ErrUpdateBuildModule = NewHTTPError(6521, "更新编译模块失败")
	// ErrListBuildModule ...
	ErrListBuildModule = NewHTTPError(6522, "列出编译模块失败")
	// ErrGetBuildModule ...
	ErrGetBuildModule = NewHTTPError(6523, "查询编译模块失败")
	// ErrDeleteBuildModule ...
	ErrDeleteBuildModule = NewHTTPError(6524, "删除构建模块失败")
	// ErrUpdateBuildParam ...
	ErrUpdateBuildParam = NewHTTPError(6525, "更新参数化配置失败")
	// ErrUpdateBuildServiceTmpls ...
	ErrUpdateBuildServiceTmpls = NewHTTPError(6526, "更新关联服务模板失败")
	// ErrConvertSubTasks ...
	ErrConvertSubTasks = NewHTTPError(6527, "转换工作流任务失败")
	// ErrConvertBuildModule ...
	ErrConvertBuildModule = NewHTTPError(6528, "转换编译模块失败")
	// ErrCreateTestModule ...
	ErrCreateTestModule = NewHTTPError(6529, "新建测试模块失败")
	// ErrUpdateTestModule ...
	ErrUpdateTestModule = NewHTTPError(6530, "更新测试模块失败")
	// ErrListTestModule ...
	ErrListTestModule = NewHTTPError(6531, "列出测试模块失败")
	// ErrGetTestModule ...
	ErrGetTestModule = NewHTTPError(6532, "获取测试模块失败")
	// ErrDeleteTestModule ...
	ErrDeleteTestModule = NewHTTPError(6533, "删除测试模块失败")

	// Workflow APIs Range: 6540 - 6550
	//-----------------------------------------------------------------------------------------------

	// ErrUpsertWorkflow ...
	ErrUpsertWorkflow = NewHTTPError(6540, "新建或更新wokflow失败")
	// ErrListWorkflow ...
	ErrListWorkflow = NewHTTPError(6541, "列出workflow失败")
	// ErrFindWorkflow ...
	ErrFindWorkflow = NewHTTPError(6542, "查询workflow失败")
	// ErrDeleteWorkflow ...
	ErrDeleteWorkflow = NewHTTPError(6543, "删除workflow失败")

	//-----------------------------------------------------------------------------------------------
	// Directory APIs Range: 6550 - 6560
	//-----------------------------------------------------------------------------------------------
	//ErrListCodehosts = NewHTTPError(6560, "列出Codehost失败")
	ErrCodehostListNamespaces = NewHTTPError(6550, "请确认是否为有效代码源，列出Namespace失败")
	ErrCodehostListProjects   = NewHTTPError(6551, "请确认是否为有效代码源，列出仓库失败")
	ErrCodehostListBranches   = NewHTTPError(6552, "请确认是否为有效代码源，列出分支失败")
	ErrCodehostListPrs        = NewHTTPError(6553, "请确认是否为有效代码源，列出pr失败")
	ErrCodehostListTags       = NewHTTPError(6554, "请确认是否为有效代码源，列出tag失败")

	//-----------------------------------------------------------------------------------------------
	// delivery_version APIs Range: 6560 - 6569
	//-----------------------------------------------------------------------------------------------

	ErrCreateDeliveryVersion = NewHTTPError(6560, "新建交付中心版本失败")
	ErrFindDeliveryVersion   = NewHTTPError(6561, "获取交付中心版本列表失败")
	ErrDeleteDeliveryVersion = NewHTTPError(6562, "删除交付中心版本失败")
	ErrGetDeliveryVersion    = NewHTTPError(6563, "查询交付中心版本失败")
	ErrFindDeliveryProducts  = NewHTTPError(6564, "查询交付中心产品列表失败")
	ErrUpdateDeliveryVersion = NewHTTPError(6565, "更新交付中心版本失败")

	//-----------------------------------------------------------------------------------------------
	// delivery_build APIs Range: 6570 - 6579
	//-----------------------------------------------------------------------------------------------
	ErrCreateDeliveryBuild = NewHTTPError(6570, "新建交付中心buildInfo失败")
	ErrFindDeliveryBuild   = NewHTTPError(6571, "获取交付中心buildnfo列表失败")
	ErrDeleteDeliveryBuild = NewHTTPError(6572, "删除交付中心buildnfo失败")
	ErrGetDeliveryBuild    = NewHTTPError(6573, "查询交付中心buildnfo失败")

	//-----------------------------------------------------------------------------------------------
	// delivery_deploy APIs Range: 6580 - 6589
	//-----------------------------------------------------------------------------------------------
	ErrCreateDeliveryDeploy = NewHTTPError(6580, "新建交付中心deploynfo失败")
	ErrFindDeliveryDeploy   = NewHTTPError(6581, "获取交付中心deploynfo失败")
	ErrDeleteDeliveryDeploy = NewHTTPError(6582, "删除交付中心deploynfo失败")
	ErrGetDeliveryDeploy    = NewHTTPError(6583, "查询交付中心deploynfo失败")

	//-----------------------------------------------------------------------------------------------
	// delivery_distribute APIs Range: 6590 - 6599
	//-----------------------------------------------------------------------------------------------
	ErrCreateDeliveryDistribute = NewHTTPError(6590, "新建交付中心distributeInfo失败")
	ErrFindDeliveryDistribute   = NewHTTPError(6591, "获取交付中心distributeInfo列表失败")
	ErrDeleteDeliveryDistribute = NewHTTPError(6592, "删除交付中心distributeInfo失败")
	ErrGetDeliveryDistribute    = NewHTTPError(6593, "查询交付中心distributeInfo失败")

	//-----------------------------------------------------------------------------------------------
	// delivery_test APIs Range: 6600 - 6609
	//-----------------------------------------------------------------------------------------------
	ErrCreateDeliveryTest = NewHTTPError(6600, "新建交付中心testInfo失败")
	ErrFindDeliveryTest   = NewHTTPError(6601, "获取交付中心testInfo列表失败")
	ErrDeleteDeliveryTest = NewHTTPError(6602, "删除交付中心testInfo失败")
	ErrGetDeliveryTest    = NewHTTPError(6603, "查询交付中心testInfo失败")

	//-----------------------------------------------------------------------------------------------
	// delivery_security APIs Range: 6620 - 6629
	//-----------------------------------------------------------------------------------------------
	ErrCreateDeliverySecurity    = NewHTTPError(6610, "新建交付中心安全扫描信息失败")
	ErrFindDeliverySecurity      = NewHTTPError(6611, "获取交付中心安全扫描列表失败")
	ErrDeleteDeliverySecurity    = NewHTTPError(6612, "删除交付中心安全扫描失败")
	ErrGetDeliverySecurity       = NewHTTPError(6613, "查询交付中心安全扫描失败")
	ErrFindDeliverySecurityStats = NewHTTPError(6614, "获取安全扫描统计结果失败")

	//-----------------------------------------------------------------------------------------------
	// S3Storage Manage APIs Range: 6630 - 6639
	//-----------------------------------------------------------------------------------------------
	ErrValidateS3Storage    = NewHTTPError(6630, "无法连接指定的对象存储块")
	ErrFindDefaultS3Storage = NewHTTPError(6631, "没有配置默认的对象存储")
	ErrS3Storage            = NewHTTPError(6632, "对象存储参数错误")
	ErrFindS3               = NewHTTPError(6633, "未找到s3的配置")
	ErrFindS3Storage        = NewHTTPError(6634, "未找到指定对象存储")

	// K8SCluster Manage APIs Range: 6640 - 6650
	//-----------------------------------------------------------------------------------------------
	ErrListK8SCluster  = NewHTTPError(6640, "列出集群列表失败")
	ErrCreateCluster   = NewHTTPError(6641, "创建集群失败")
	ErrUpdateCluster   = NewHTTPError(6642, "更新集群失败")
	ErrClusterNotFound = NewHTTPError(6643, "未找到指定集群")
	ErrDeleteCluster   = NewHTTPError(6644, "删除集群失败")

	//-----------------------------------------------------------------------------------------------
	// operation APIs Range: 6650 - 6659
	//-----------------------------------------------------------------------------------------------
	ErrCreateOperationLog    = NewHTTPError(6651, "添加操作日志失败")
	ErrFindOperationLog      = NewHTTPError(6652, "获取操作日志列表失败")
	ErrFindOperationLogCount = NewHTTPError(6653, "获取操作日志总数失败")
	ErrUpdateOperationLog    = NewHTTPError(6654, "更新操作日志失败")

	//-----------------------------------------------------------------------------------------------
	// operation APIs Range: 6660 - 6669
	//-----------------------------------------------------------------------------------------------
	ErrCreateArtifact       = NewHTTPError(6661, "添加交付信息失败")
	ErrFindArtifact         = NewHTTPError(6662, "获取交付物信息失败")
	ErrFindArtifacts        = NewHTTPError(6663, "获取交付物列表失败")
	ErrCreateActivity       = NewHTTPError(6664, "添加交付事件失败")
	ErrFindActivities       = NewHTTPError(6665, "获取交付事件列表失败")
	ErrCreateArtifactFailed = NewHTTPError(6666, "该交付物已经存在")

	//-----------------------------------------------------------------------------------------------
	// basicImage APIs Range: 6670 - 6679
	//-----------------------------------------------------------------------------------------------
	ErrGetBasicImage        = NewHTTPError(6671, "获取基础镜像失败")
	ErrCreateBasicImage     = NewHTTPError(6672, "创建基础镜像失败")
	ErrUpdateBasicImage     = NewHTTPError(6673, "更新基础镜像失败")
	ErrListBasicImages      = NewHTTPError(6674, "列出基础镜像失败")
	ErrDeleteBasicImage     = NewHTTPError(6675, "删除基础镜像失败")
	ErrDeleteUsedBasicImage = NewHTTPError(6676, "删除基础镜像失败，此基础镜像已经被引用，请确认")

	//-----------------------------------------------------------------------------------------------
	// privateKey APIs Range: 6680 - 6689
	//-----------------------------------------------------------------------------------------------
	ErrGetPrivateKey        = NewHTTPError(6681, "获取私钥失败")
	ErrCreatePrivateKey     = NewHTTPError(6682, "创建私钥失败")
	ErrUpdatePrivateKey     = NewHTTPError(6683, "更新私钥失败")
	ErrListPrivateKeys      = NewHTTPError(6684, "列出私钥失败")
	ErrDeletePrivateKey     = NewHTTPError(6685, "删除私钥失败")
	ErrDeleteUsedPrivateKey = NewHTTPError(6686, "删除私钥失败，此私钥已经被引用，请确认")

	//-----------------------------------------------------------------------------------------------
	// signature APIs Range: 6690 - 6699
	//-----------------------------------------------------------------------------------------------
	ErrCreateSignature = NewHTTPError(6691, "添加或更新License失败")
	ErrDeleteSignature = NewHTTPError(6692, "删除License失败")
	ErrListSignatures  = NewHTTPError(6693, "列出License失败")

	//-----------------------------------------------------------------------------------------------
	// sonar APIs Range: 6700 - 6800
	//-----------------------------------------------------------------------------------------------
	ErrRepositoryList             = NewHTTPError(6701, "列出repo失败")
	ErrRepoQualityGateFind        = NewHTTPError(6702, "查询RepoQualityGate失败")
	ErrCollectCodeCoverage        = NewHTTPError(6703, "搜集代码覆盖率数据失败")
	ErrListCodeCoverageDetails    = NewHTTPError(6704, "列出代码覆盖率详情失败")
	ErrGetDeliveryMeasureInfo     = NewHTTPError(6705, "获取交付度量失败")
	ErrAnalyzeSonar               = NewHTTPError(6706, "Sonar分析失败")
	ErrCollectSonar               = NewHTTPError(6707, "收集Sonar数据失败")
	ErrListIssueMeasures          = NewHTTPError(6708, "列出IssueMeasure失败")
	ErrListSecurityMeasureDetails = NewHTTPError(6709, "列出SecurityMeasureDetail失败")
	ErrListSecurityMeasureCount   = NewHTTPError(6710, "列出SecurityMeasureCount失败")
	ErrListMeasuresByTeam         = NewHTTPError(6711, "列出团队Measures失败")
	ErrListMeasuresByOrg          = NewHTTPError(6712, "列出组织Measures失败")
	ErrListMeasuresByRepo         = NewHTTPError(6713, "列出repo Measures失败")
	ErrListMeasuresByProduct      = NewHTTPError(6714, "列出项目Measures失败")
	ErrGetProductMeasureByOrg     = NewHTTPError(6715, "获取组织ProductMeasure失败")
	ErrGetMeasureHistoryByRepo    = NewHTTPError(6716, "获取团队MeasureHistory失败")
	ErrMeasureTranslation         = NewHTTPError(6717, "获取MeasureTranslation失败")
	ErrListMeasureData            = NewHTTPError(6718, "列出MeasureData失败")
	ErrGetIndexInMeasure          = NewHTTPError(6719, "获取measure index失败")
	ErrUpdateIndexInMeasure       = NewHTTPError(6720, "更新measure index失败")
	ErrUpdateQualityGates         = NewHTTPError(6721, "更新QualityGates失败")
	ErrQueryCIScript              = NewHTTPError(6722, "查询CI脚本失败")
	ErrGetTeamQualityGates        = NewHTTPError(6723, "获取团队QualityGates失败")
	ErrGetProductQualityGates     = NewHTTPError(6724, "获取项目QualityGates失败")
	ErrUpdateProductQualityGates  = NewHTTPError(6725, "更新项目QualityGates失败")
	ErrGetPublicScripts           = NewHTTPError(6726, "获取公开脚本失败")
	ErrGetRepositories            = NewHTTPError(6727, "获取仓库失败失败")
	ErrExtendRepos                = NewHTTPError(6728, "扩展仓库数据数据失败")
	ErrGetRepositoryNotInCi       = NewHTTPError(6729, "获取非CI仓库失败")
	ErrGetRepoNamespace           = NewHTTPError(6730, "获取RepoNamespace失败")
	ErrModifyRepo                 = NewHTTPError(6731, "更新仓库失败")
	ErrListProductRepos           = NewHTTPError(6732, "列出项目仓库失败")
	ErrListTeamRepos              = NewHTTPError(6733, "列出团队仓库失败")
	ErrModifyRepoInTeam           = NewHTTPError(6734, "更新团队仓库失败")
	ErrRemoveTeamRepos            = NewHTTPError(6735, "移除团队仓库失败")
	ErrSyncCodehost               = NewHTTPError(6736, "同步codehost失败")
	ErrRemoveRepos                = NewHTTPError(6737, "移除仓库失败")
	ErrGetBuildDetails            = NewHTTPError(6738, "获取构建详情失败")
	ErrGetMeasureInfo             = NewHTTPError(6739, "获取度量信息失败")
	ErrPullTestsMeasure           = NewHTTPError(6740, "拉取持续交付数据失败")
	ErrPullDeliveryMeasure        = NewHTTPError(6741, "拉取持续部署数据失败")
	ErrPullRepos                  = NewHTTPError(6742, "同步代码库失败")

	//-----------------------------------------------------------------------------------------------
	// proxy APIs Range: 6800 - 6809
	//-----------------------------------------------------------------------------------------------
	ErrGetProxy         = NewHTTPError(6801, "获取代理失败")
	ErrCreateProxy      = NewHTTPError(6802, "创建代理失败")
	ErrUpdateProxy      = NewHTTPError(6803, "更新代理失败")
	ErrListProxies      = NewHTTPError(6804, "列出代理失败")
	ErrDeleteProxy      = NewHTTPError(6805, "删除代理失败")
	ErrTestConnection   = NewHTTPError(6806, "代理连接测试失败")
	ErrForwardOperation = NewHTTPError(6807, "上报数据转发失败")

	//-----------------------------------------------------------------------------------------------
	// Cronjob Error Range: 6810 - 6819
	//-----------------------------------------------------------------------------------------------
	ErrUpsertCronjob = NewHTTPError(6810, "更新定时器失败")

	//-----------------------------------------------------------------------------------------------
	// dindClean Error Range: 6820 - 6829
	//-----------------------------------------------------------------------------------------------
	ErrDindClean       = NewHTTPError(6820, "系统正在清理中，请等待...")
	ErrCreateDindClean = NewHTTPError(6821, "创建镜像缓存清理失败")
	ErrUpdateDindClean = NewHTTPError(6822, "更新镜像缓存清理失败")

	//-----------------------------------------------------------------------------------------------
	// jenkins integraton Error Range: 6830 - 6839
	//-----------------------------------------------------------------------------------------------
	ErrCreateJenkinsIntegration = NewHTTPError(6831, "创建jenkins集成失败")
	ErrListJenkinsIntegration   = NewHTTPError(6832, "获取jenkins集成列表失败")
	ErrUpdateJenkinsIntegration = NewHTTPError(6833, "更新jenkins集成失败")
	ErrDeleteJenkinsIntegration = NewHTTPError(6834, "删除jenkins集成失败")
	ErrTestJenkinsConnection    = NewHTTPError(6835, "用户名或者密码不正确")
	ErrListJobNames             = NewHTTPError(6836, "获取job名称列表失败")
	ErrListJobBuildArgs         = NewHTTPError(6837, "获取job构建参数列表失败")
)
