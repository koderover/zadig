<template>
  <div class="not-k8s-config-container" v-loading="loading"
       @scroll="onScroll">
    <!--Host-create-dialog-->
    <div class="anchor-container">
      <el-tabs v-model="anchorTab"
               @tab-click="goToAnchor"
               tab-position="right">
        <el-tab-pane label="基本信息"></el-tab-pane>
        <el-tab-pane label="构建脚本"></el-tab-pane>
        <el-tab-pane label="资源配置"></el-tab-pane>
        <el-tab-pane label="部署配置"></el-tab-pane>
        <el-tab-pane label="探活配置"></el-tab-pane>
      </el-tabs>
    </div>
    <div id="基本信息"
         class="section scroll">
      <el-form ref="addConfigForm"
               :model="buildConfig"
               :rules="createRules"
               label-position="left"
               label-width="90px">

        <el-row>
          <el-col :span="7">
            <el-form-item prop="service_name"
                          label="服务名称">
              <el-input v-model="buildConfig.service_name"
                        placeholder="服务名称"
                        autofocus
                        size="small"
                        :disabled="isEdit"
                        auto-complete="off"></el-input>
            </el-form-item>

          </el-col>

          <el-col :span="8" style="margin-left: 10px;">
            <el-form-item label="构建超时">
              <el-input-number size="mini"
                               :min="1"
                               v-model="buildConfig.timeout"></el-input-number>
              <span style="margin-left: 10px;"> 分钟</span>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row>
          <el-col :span="8">
            <el-form-item label="构建名称"
                          prop="name">
              <el-select v-model="buildConfig.name"
                         @change="syncBuild"
                         size="small"
                         placeholder="请选择"
                         filterable
                         :allow-create="!isEdit">
                <el-option v-for="item in builds"
                           filterable
                           :key="item.id"
                           :label="item.name"
                           :value="item.name">
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <span class="item-title">构建环境</span>
        <div class="divider item"></div>
        <el-row :gutter="30">
          <el-col :span="10">
            <el-form-item prop="pre_build.image_id"
                          label="操作系统">
              <el-select size="small"
                         style="width: 200px;"
                         v-model="buildConfig.pre_build.image_id"
                         placeholder="请选择">
                <el-option v-for="(sys,index) in systems"
                           :key="index"
                           :label="sys.label"
                           :value="sys.id">
                  <span> {{sys.label}}
                    <el-tag v-if="sys.image_from==='custom'"
                            type="info"
                            size="mini"
                            effect="light">
                      自定义
                    </el-tag>
                  </span>
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="10">
              <el-form-item label="资源规格">

                  <el-select size="small"
                            v-model="buildConfig.pre_build.res_req"
                            placeholder="请选择">
                    <el-option label="高 | CPU: 16 核 内存: 32 GB"
                              value="high">
                    </el-option>
                    <el-option label="中 | CPU: 8 核 内存: 16 GB"
                              value="medium">
                    </el-option>
                    <el-option label="低 | CPU: 4 核 内存: 8 GB"
                              value="low">
                    </el-option>
                    <el-option label="最低 | CPU: 2 核 内存: 2 GB"
                              value="min">
                    </el-option>
                  </el-select>
                </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <el-form ref="buildApp"
               :inline="true"
               :model="buildConfig"
               class="form-style1"
               label-position="top"
               label-width="80px">
        <span class="item-title">应用列表</span>
        <el-button v-if="buildConfig.pre_build.installs.length===0"
                   style="padding: 0;"
                   @click="addFirstBuildApp()"
                   type="text">新增</el-button>
        <div class="divider item"></div>
        <el-row v-for="(app,build_app_index) in buildConfig.pre_build.installs"
                :key="build_app_index">
          <el-col :span="6">
            <el-form-item
                          :prop="'pre_build.installs.' + build_app_index + '.name'"
                          :rules="{required: true, message: '应用名不能为空', trigger: 'blur'}">
              <el-select style="width: 100%;"
                         v-model="buildConfig.pre_build.installs[build_app_index]"
                         placeholder="请选择应用"
                         size="small"
                         value-key="id"
                         filterable>
                <el-option v-for="(app, index) in allApps"
                           :key="index"
                           :label="`${app.name} ${app.version} `"
                           :value="{'name':app.name,'version':app.version,'id':app.name+app.version}">
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item >
              <div class="app-operation">
                <el-button v-if="buildConfig.pre_build.installs.length >= 1"
                           @click="deleteBuildApp(build_app_index)"
                           type="danger"
                           size="small"
                           plain>删除</el-button>
                <el-button v-if="build_app_index===buildConfig.pre_build.installs.length-1"
                           @click="addBuildApp(build_app_index)"
                           type="primary"
                           size="small"
                           plain>新增</el-button>
              </div>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </div>
    <div class="section">
      <repo-select :config="buildConfig"
                   :validObj="validObj"
                   showDivider
                   showFirstLine></repo-select>
    </div>
    <div class="section">
      <el-form ref="buildEnv"
               :inline="true"
               :model="buildConfig"
               class="form-style1"
               label-position="top"
               label-width="80px">
        <span class="item-title">环境变量</span>
        <el-button v-if="buildConfig.pre_build.envs.length===0"
                   style="padding: 0;"
                   @click="addFirstBuildEnv()"
                   type="text">新增</el-button>
        <div class="divider item"></div>
        <el-row v-for="(app,build_env_index) in buildConfig.pre_build.envs"
                :key="build_env_index">
          <el-col :span="4">
            <el-form-item
                          :prop="'pre_build.envs.' + build_env_index + '.key'"
                          :rules="{required: true, message: '键 不能为空', trigger: 'blur'}">
              <el-input placeholder="键" v-model="buildConfig.pre_build.envs[build_env_index].key"
                        size="small">
              </el-input>
            </el-form-item>
          </el-col>
          <el-col :span="4">
            <el-form-item
                          :prop="'pre_build.envs.' + build_env_index + '.value'"
                          :rules="{required: true, message: '值 不能为空', trigger: 'blur'}">
              <el-input placeholder="值" v-model="buildConfig.pre_build.envs[build_env_index].value"
                        size="small">
              </el-input>
            </el-form-item>
          </el-col>
          <el-col :span="3">
            <el-form-item prop="is_credential">
              <el-checkbox v-model="buildConfig.pre_build.envs[build_env_index].is_credential">
                敏感信息
                <el-tooltip effect="dark"
                            content="在日志中将被隐藏"
                            placement="top">
                  <i class="el-icon-question"></i>
                </el-tooltip>
              </el-checkbox>
            </el-form-item>
          </el-col>
          <el-col :span="5">
            <el-form-item >
              <div class="app-operation">
                <el-button v-if="buildConfig.pre_build.envs.length >= 1"
                           @click="deleteBuildEnv(build_env_index)"
                           type="danger"
                           size="small"
                           plain>删除</el-button>
                <el-button v-if="build_env_index===buildConfig.pre_build.envs.length-1"
                           @click="addBuildEnv(build_env_index)"
                           type="primary"
                           size="small"
                           plain>新增</el-button>

              </div>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </div>

    <div class="section">
      <el-form ref="cacheDir"
               :inline="true"
               :model="buildConfig"
               class="form-style1"
               label-position="left"
               label-width="130px">
        <span class="item-title">缓存策略</span>
        <div class="divider item"></div>
        <el-row>
          <el-col :span="4">
            <el-form-item label="使用工作空间缓存">
              <el-switch  v-model="useWorkspaceCache"
                         active-color="#409EFF">
              </el-switch>
            </el-form-item>
          </el-col>
        </el-row>
        <template >
          <el-row>
            <el-col :span="4">
              <el-form-item label="缓存自定义目录">
                <el-button v-if="!this.buildConfig.caches||this.buildConfig.caches.length ===0"
                           style="padding: 0;"
                           @click="addFirstCacheDir()"
                           type="text">新增</el-button>
              </el-form-item>
            </el-col>
          </el-row>
          <el-row v-for="(dir,index) in buildConfig.caches"
                  :key="index">
            <el-col :span="10">
              <el-form-item :label="index===0?'':''">
                <el-input v-model="buildConfig.caches[index]"
                          style="width: 100%;"
                          size="small">
                  <template slot="prepend">$WORKSPACE/</template>
                </el-input>
              </el-form-item>
            </el-col>
            <el-col :span="10">
              <el-form-item :label="index===0?'':''">
                <div class="app-operation">
                  <el-button v-if="buildConfig.caches.length >= 1"
                             @click="deleteCacheDir(index)"
                             type="danger"
                             size="small"
                             plain>删除</el-button>
                  <el-button v-if="index===buildConfig.caches.length-1"
                             @click="addCacheDir(index)"
                             type="primary"
                             size="small"
                             plain>新增</el-button>
                </div>
              </el-form-item>
            </el-col>
          </el-row>
        </template>
      </el-form>
    </div>
    <div id="构建脚本"
         class="section scroll">
      <el-form ref="buildScript"
               :model="buildConfig"
               label-position="left"
               label-width="80px">
        <span class="item-title">构建脚本</span>
         <el-tooltip effect="dark"  placement="top-start">
            <div slot="content" >
                  当前可用环境变量如下，可在构建脚本中进行引用<br>
                  $WORKSPACE  工作目录<br>
                  $IMAGE &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;输出镜像名称<br>
                  $SERVICE&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;构建的服务名称<br>
                  $DIST_DIR   &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; 构建出的 Tar 包的目的目录<br>
                  $PKG_FILE   &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; 构建出的 Tar 包名称<br>
                  $ENV_NAME  &nbsp;&nbsp;&nbsp; 执行的环境名称 <br>
                  $BUILD_URL &nbsp;&nbsp;&nbsp;  构建任务的 URL<br>
                  &lt;REPONAME&gt;_PR  构建过程中指定代码仓库使用的 Pull Request 信息<br>
                  &lt;REPONAME&gt;_BRANCH  构建过程中指定代码仓库使用的分支信息<br>
                  &lt;REPONAME&gt;_TAG  构建过程中指定代码仓库使用 Tag 信息
            </div>
          <span class="variable">变量</span>
        </el-tooltip>
        <div class="divider item"></div>
        <el-row>
          <el-col :span="24"
                  class="deploy-script">
            <Resize :height="'150px'">
              <editor v-model="buildConfig.scripts"
                    lang="sh"
                    theme="xcode"
                    :options="editorOption"
                    width="100%"
                    height="100%"></editor>
            </Resize>
          </el-col>
        </el-row>
      </el-form>
      <el-form v-if="docker_enabled"
               :model="buildConfig.post_build.docker_build"
               :rules="docker_rules"
               ref="docker_build"
               class="docker label-at-left">

        <div class="extra-build-container">
          <span class="title">镜像构建
            <el-button type="text"
                       @click="removeDocker"
                       icon="el-icon-delete"></el-button>
          </span>
          <el-form-item label="镜像构建目录："
                        prop="work_dir">
            <el-input v-model="buildConfig.post_build.docker_build.work_dir"
                      size="small">
              <template slot="prepend">$WORKSPACE/</template>
            </el-input>
          </el-form-item>
          <el-form-item label="Dockerfile 文件的完整路径："
                        prop="docker_file">
            <el-input v-model="buildConfig.post_build.docker_build.docker_file"
                      size="small">
              <template slot="prepend">$WORKSPACE/</template>
            </el-input>
          </el-form-item>
          <el-form-item label="镜像构建参数：">
            <el-tooltip effect="dark"
                        content="支持所有 Docker Build 参数"
                        placement="top-start">
              <el-input v-model="buildConfig.post_build.docker_build.build_args"
                        size="small"
                        placeholder="--build-arg key=value"></el-input>
            </el-tooltip>
          </el-form-item>
        </div>
        <div class="divider">
        </div>

      </el-form>
      <el-form v-if="binary_enabled"
               :model="buildConfig.post_build.file_archive"
               :rules="file_archive_rules"
               ref="file_archive"
               class="stcov label-at-left">

        <div class="extra-build-container">
          <span class="title">二进制包归档
            <el-button type="text"
                       @click="removeBinary"
                       icon="el-icon-delete"></el-button>
          </span>
          <el-form-item label="二进制包存放路径："
                        prop="file_location">
            <el-input v-model="buildConfig.post_build.file_archive.file_location"
                      size="small">
              <template slot="append">/$PKG_FILE</template>
              <template slot="prepend">$WORKSPACE/</template>
            </el-input>
          </el-form-item>
        </div>
        <div class="divider">
        </div>
      </el-form>
      <div class="section add-extra-step">
        <el-dropdown @command="addExtra">
          <el-button size="small">
            新增构建步骤<i class="el-icon-arrow-down el-icon--right"></i>
          </el-button>
          <el-dropdown-menu slot="dropdown">
            <el-dropdown-item command="docker"
                              :disabled="docker_enabled">镜像构建</el-dropdown-item>
            <el-dropdown-item command="binary"
                              :disabled="binary_enabled">二进制包归档</el-dropdown-item>

          </el-dropdown-menu>
        </el-dropdown>
      </div>
      <div id="资源配置"
           class="section scroll">
        <el-form :model="pmService"
                 :inline="true"
                 ref="envConfig"
                 label-width="120px"
                 label-position="top">
          <span class="item-title">资源配置</span>
          <div class="divider item"></div>
          <div class="">
            <el-row v-for="(item,item_index) in pmService.env_configs"
                    class="env-config"
                    :key="item_index">
              <el-col :span="3">
                <el-form-item :label="item_index===0?'环境':''"
                              :prop="'env_configs.'+item_index+'.env_name'"
                              :rules="{required: false, message: '请选择部署环境', trigger: 'blur'}">
                  <div class="env-name">{{item.env_name}}</div>
                </el-form-item>
              </el-col>
              <el-col :span="5">
                <div class="">
                  <el-form-item :label="item_index===0?'主机':''"
                                :prop="'env_configs.'+item_index+'.host_ids'"
                                :rules="{required: false, message: '请选择主机', trigger: 'blur'}">
                    <el-button v-if="allHost.length===0"
                               @click="createHost"
                               type="text">创建主机</el-button>
                    <el-select v-else
                               v-model="item.host_ids"
                               size="small"
                               multiple
                               placeholder="请选择主机">
                      <el-option v-for="(host,index) in  allHost"
                                 :key="index"
                                 :label="`${host.name}-${host.ip}`"
                                 :value="host.id">
                      </el-option>
                    </el-select>
                  </el-form-item>
                </div>
              </el-col>
              <el-col v-if="!isOnboarding"
                      :span="8">
                <el-form-item :label="item_index===0?'操作':''">
                  <div class="app-operation">
                    <el-button v-if="item.showDelete"
                               @click="deleteEnvConfig(item_index)"
                               type="danger"
                               icon="el-icon-delete"
                               size="mini"
                               circle></el-button>
                    <el-tooltip v-else
                                effect="dark"
                                content="环境已存在，不可删除配置"
                                placement="top">
                      <span><i class="el-icon-question"></i></span>
                    </el-tooltip>
                  </div>
                </el-form-item>
              </el-col>
            </el-row>
          </div>
        </el-form>
      </div>
      <div id="部署配置"
           class="section scroll">
        <el-form ref="deploy-env"
                 :inline="true"
                 :model="buildConfig"
                 class="form-style1"
                 label-position="left"
                 label-width="80px">
          <span class="item-title">部署配置</span>
          <div class="divider item"></div>

          <div class="deploy-method">
            <el-radio-group v-model="useSshKey">
              <el-radio :label="false">本地直连部署
                <el-tooltip placement="top">
                  <div slot="content">
                    本地直连部署：需确保本系统能连通或访问到脚本中的主机地址(含本机)
                  </div>
                  <i class="icon el-icon-question"></i>
                </el-tooltip>
              </el-radio>
              <el-radio :label="true">使用 SSH Agent 远程部署
                <el-tooltip placement="top">
                  <div slot="content">
                    使用 SSH Agent 远程部署：安全登陆到目标机器，执行部署操作，可在系统设置-主机管理中进行配置
                  </div>
                  <i class="icon el-icon-question"></i>
                </el-tooltip>
              </el-radio>
            </el-radio-group>
            <el-select v-if="useSshKey"
                       v-model="buildConfig.sshs"
                       size="mini"
                       multiple
                       placeholder="请选择主机">
              <el-option v-for="(item,index) in  allHost"
                         :key="index"
                         :label="item.name"
                         :value="item.id">
              </el-option>
            </el-select>
          </div>
          <el-row>
            <el-col :span="24"
                    class="deploy-script">
              <editor v-model="buildConfig.pm_deploy_scripts"
                      lang="sh"
                      theme="xcode"
                      :options="editorOption"
                      width="100%"
                      height="300px">
              </editor>
            </el-col>
          </el-row>
        </el-form>
      </div>
      <div id="探活配置"
           class="section scroll">
        <el-form ref="deploy"
                 :inline="true"
                 :model="pmService"
                 class="form-style1"
                 label-position="left"
                 label-width="80px">
          <el-row>
            <el-col :span="4">
              <el-form-item label="探活配置">
                <el-switch @change="checkEnvConfig"
                           v-model="check_status_enabled"
                           active-color="#409EFF">
                </el-switch>
              </el-form-item>
            </el-col>
          </el-row>
          <template v-if="check_status_enabled">
            <el-form :model="pmService"
                     :inline="true"
                     ref="healthCheck"
                     label-width="120px"
                     label-position="left">
              <el-card class="health-check-card"
                       v-for="(item,item_index) in pmService.health_checks"
                       :key="item_index">
                <div slot="header"
                     class="clearfix">
                  <el-button type="danger"
                             class="delete-btn"
                             icon="el-icon-delete"
                             plain
                             size="mini"
                             @click="deleteCheck(item_index)"
                             circle></el-button>
                </div>
                <el-row class="healthcheck-item">
                  <el-form-item label="协议："
                                :prop="'health_checks.'+item_index+'.protocol'"
                                :rules="{required: true, message: '请选择协议', trigger: 'blur'}">
                    <el-select v-model="item.protocol"
                               style="width: 179px;"
                               size="small"
                               placeholder="请选择协议">
                      <el-option label="HTTP"
                                 value="http">
                      </el-option>
                      <el-option label="HTTPS"
                                 value="https">
                      </el-option>
                      <el-option label="TCP"
                                 value="tcp">
                      </el-option>
                    </el-select>

                  </el-form-item>
                </el-row>
                <el-row v-if="item.protocol==='http'||item.protocol==='https'"
                        class="healthcheck-item">
                  <el-form-item label="路径："
                                :rules="{type: 'string',message: '请填写路径',required: false,trigger: ['blur', 'change']}"
                                :prop="'health_checks.'+item_index+'.path'">
                    <el-input v-model="item.path"
                              placeholder="example.com/index.html"
                              size="small">
                    </el-input>
                  </el-form-item>
                </el-row>
                <el-row class="healthcheck-item">
                  <el-form-item label="端口："
                                :rules="{type: 'number',required: false,validator: validateHealthCheckPort,trigger: ['blur', 'change']}"
                                :prop="'health_checks.'+item_index+'.port'">
                    <el-input v-model.number="item.port"
                              placeholder="1-65535"
                              size="small">
                    </el-input>
                  </el-form-item>
                </el-row>
                <el-row class="healthcheck-item">
                  <el-form-item label="响应超时："
                                :rules=" {type: 'number',required: true,validator: validateHealthCheckTimeout,trigger: ['blur', 'change']}"
                                :prop="'health_checks.'+item_index+'.time_out'">
                    <el-input v-model.number="item.time_out"
                              placeholder="2(2-60) 秒"
                              size="small">
                    </el-input>
                  </el-form-item>
                </el-row>
                <el-button type="primary"
                           size="mini"
                           round
                           plain
                           :icon="showCheckStatusAdvanced[item_index]?'el-icon-arrow-up':'el-icon-arrow-down'"
                           @click="changeAdvancedShow(item_index)">
                  高级设置
                </el-button>
                <template v-if="showCheckStatusAdvanced[item_index]">
                  <el-row class="healthcheck-item">
                    <el-form-item label="探测间隔："
                                  :rules="{type: 'number',required: false,validator: validateHealthCheckInterval,trigger: ['blur', 'change']}"
                                  :prop="'health_checks.'+item_index+'.interval'">
                      <el-input v-model.number="item.interval"
                                placeholder="2(2-60) 秒"
                                size="small">
                      </el-input>
                    </el-form-item>
                  </el-row>
                  <el-row class="healthcheck-item">
                    <el-form-item label="健康阈值："
                                  :rules="{ type: 'number',required: false,validator: validateHealthCheckThreshold,trigger: ['blur', 'change']}"
                                  :prop="'health_checks.'+item_index+'.healthy_threshold'">
                      <span slot="label">健康阈值
                        <el-tooltip effect="dark"
                                    placement="top">
                          <div slot="content">从不健康变为健康的连续探测次数</div>
                          <i class="el-icon-question"></i>
                        </el-tooltip>
                        ：
                      </span>
                      <el-input v-model.number="item.healthy_threshold"
                                placeholder="2(2-10) 次"
                                size="small">
                      </el-input>
                    </el-form-item>
                  </el-row>
                  <el-row class="healthcheck-item">
                    <el-form-item label="不健康阈值："
                                  :rules=" {type: 'number',required: false,validator: validateHealthCheckThreshold,trigger: ['blur', 'change']}"
                                  :prop="'health_checks.'+item_index+'.unhealthy_threshold'">
                      <span slot="label">不健康阈值
                        <el-tooltip effect="dark"
                                    placement="top">
                          <div slot="content">从健康变为不健康的连续探测次数</div>
                          <i class="el-icon-question"></i>
                        </el-tooltip>
                        ：
                      </span>
                      <el-input v-model.number="item.unhealthy_threshold"
                                placeholder="2(2-10) 次"
                                size="small">
                      </el-input>
                    </el-form-item>
                  </el-row>
                </template>
              </el-card>
              <div class="add-check">
                <el-button type="primary"
                           icon="el-icon-circle-plus-outline"
                           plain
                           size="mini"
                           @click="addCheck()"
                           circle></el-button>
              </div>

              <div class="divider">
              </div>
            </el-form>
          </template>
        </el-form>
      </div>
    </div>
    <div class="save-btn">
      <el-button type="primary" @click="savePmService">
          保存
       </el-button>
    </div>
  </div>
</template>
<script>
import { listProductAPI, serviceTemplateAPI, getBuildConfigsAPI, getBuildConfigDetailAPI, getAllAppsAPI, getImgListAPI, getCodeSourceAPI, createPmServiceAPI, updatePmServiceAPI, getHostListAPI, createHostAPI } from '@api'
import aceEditor from 'vue2-ace-bind'
import ValidateSubmit from '@utils/validate_async'
import Resize from '@/components/common/resize.vue'
const validateServiceName = (rule, value, callback) => {
  if (value === '') {
    callback(new Error('请输入服务名称'))
  } else {
    if (!/^[a-z0-9-]+$/.test(value)) {
      callback(new Error('名称只支持小写字母和数字，特殊字符只支持中划线'))
    } else {
      callback()
    }
  }
}
export default {
  props: {
    isEdit: Boolean,
    serviceName: String,
    changeUpdateEnvDisabled: Function
  },
  data () {
    return {
      loading: false,
      buildResource: [
        {
          label: '高',
          value: 'high',
          desc: 'CPU: 16 核 内存: 32 GB'
        },
        {
          label: '中',
          value: 'medium',
          desc: 'CPU: 8 核 内存: 16 GB'
        },
        {
          label: '低',
          value: 'low',
          desc: 'CPU: 4 核 内存: 8 GB'
        },
        {
          label: '最低',
          value: 'min',
          desc: 'CPU: 2 核 内存: 2 GB'
        }
      ],
      anchorTab: '',
      serviceList: [],
      builds: [],
      useSshKey: false,
      dialogServiceListVisible: false,
      dialogHostCreateFormVisible: false,
      host: {
        name: '',
        label: '',
        ip: '',
        user_name: '',
        private_key: ''
      },
      editorOption: {
        enableEmmet: true,
        showLineNumbers: true,
        showFoldWidgets: true,
        showGutter: false,
        displayIndentGuides: false,
        showPrintMargin: false
      },
      pmService: {
        service_name: '',
        health_checks: [{
          protocol: 'http',
          path: '',
          time_out: null,
          interval: null,
          healthy_threshold: null,
          unhealthy_threshold: null
        }],
        env_configs: []
      },
      buildConfig: {
        service_name: '',
        version: 'stable',
        name: '',
        desc: '',
        repos: [],
        caches: [],
        timeout: 60,
        pre_build: {
          clean_workspace: false,
          res_req: 'low',
          build_os: 'xenial',
          image_id: '',
          image_from: '',
          installs: [],
          envs: [],
          enable_proxy: false,
          enable_gocov: false,
          parameters: []
        },
        scripts: '#!/bin/bash\nset -e',
        main_file: '',
        post_build: {
        },
        pm_deploy_scripts: "#<------------------------------------------------------------------------------->\n## 构建脚本中的变量均可使用，其他内置可用变量如下\n## ENV_NAME               环境名称，用于区分不同的集成环境，系统内置集成环境：dev，qa\n## <AGENT_NAME>_PK        通过 SSH Agent 远程登录服务器使用的私钥 id_rsa，其中 AGENT_NAME 为 SSH AGENT 名称，使用时需要自己填写完整  \n## <AGENT_NAME>_USERNAME  通过 SSH Agent 远程登录到服务器的用户名称 \n## <AGENT_NAME>_IP        SSH Agent 目标服务器的 IP 地址\n## 远程部署时，可以通过使用命令 `ssh -i $<AGENT_NAME>_PK $<AGENT_NAME>_USERNAME@$<AGENT_NAME>_IP '自定义脚本'` 进行部署操作\n#<------------------------------------------------------------------------------->\n#!/bin/bash\nset -e",
        sshs: null
      },
      stcov_enabled: false,
      docker_enabled: false,
      binary_enabled: false,
      check_status_enabled: false,
      editBuildConfigName: false,
      showCheckStatusAdvanced: {},
      allApps: [],
      allCodeHosts: [],
      allHost: [],
      envNameList: [],
      codeInfo: {},
      createRules: {
        service_name: [
          {
            type: 'string',
            required: true,
            validator: validateServiceName,
            trigger: 'change'
          }
        ],
        name: [
          {
            type: 'string',
            required: true,
            message: '请输入构建名称或者选择已有构建',
            trigger: 'change'
          }
        ],
        'pre_build.image_id': [
          {
            type: 'string',
            required: true,
            message: '请选择系统',
            trigger: 'change'
          }
        ]
      },
      docker_rules: {
        work_dir: [
          {
            type: 'string',
            message: '请填写镜像构建目录',
            required: true,
            trigger: 'blur'
          }
        ],
        docker_file: [
          {
            type: 'string',
            message: '请填写Dockerfile路径',
            required: true,
            trigger: 'blur'
          }
        ]
      },
      file_archive_rules: {
        file_location: [
          {
            type: 'string',
            message: '请填写文件路径',
            required: true,
            trigger: 'blur'
          }
        ]
      },
      addHostRules: {
        name: [{
          type: 'string',
          required: true,
          message: '请输入主机名称',
          trigger: 'change'
        }],
        label: [{
          type: 'string',
          required: false,
          message: '请输入主机标签',
          trigger: 'change'
        }],
        user_name: [{
          type: 'string',
          required: true,
          message: '请输入用户名'
        }],
        ip: [{
          type: 'string',
          required: true,
          message: '请输入主机 IP'
        }],
        private_key: [{
          type: 'string',
          required: true,
          message: '请输入私钥'
        }]
      },
      systems: [],
      validObj: new ValidateSubmit()
    }
  },
  methods: {
    jumpProject (projectName) {
      if (!this.isOnboarding) {
        this.$router.push(`/v1/projects/detail/${projectName}/services`)
        return
      }
      this.$confirm('确认跳出后就不再进入 onboarding 流程。', '确认跳出产品交付向导？', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        this.saveOnboardingStatus(this.projectName, 0).then((res) => {
          this.$router.push(`/v1/projects/detail/${projectName}/services`)
        }).catch(() => {
          this.$message.info('跳转失败')
        })
      }).catch(() => {
        this.$message.info('取消跳转')
      })
    },
    onScroll (e) {
      const scrollItems = document.querySelectorAll('.scroll')
      for (let i = scrollItems.length - 1; i >= 0; i--) {
        // 判断滚动条滚动距离是否大于当前滚动项可滚动距离
        const judge =
          e.target.scrollTop >=
          scrollItems[i].offsetTop - scrollItems[0].offsetTop - 150
        if (judge) {
          this.anchorTab = i.toString()
          break
        }
      }
    },
    goToAnchor (id) {
      this.$nextTick(() => {
        document.querySelector(`#${id.label}`).scrollIntoView({
          behavior: 'smooth',
          block: 'start'
        })
      })
    },
    syncBuild (val) {
      if (val) {
        const findItem = this.builds.find((element) => {
          return element.name === val
        })
        if (findItem) {
          this.syncBuildConfig(val, this.buildConfigVersion, this.projectName)
        }
      }
    },
    syncBuildConfig (buildName, version, projectName) {
      getBuildConfigDetailAPI(buildName, version, projectName).then((response) => {
        response.pre_build.installs.forEach(element => {
          element.id = element.name + element.version
        })
        let originServiceName = ''
        if (!this.isEdit) {
          originServiceName = this.buildConfig.service_name
        } else if (this.isEdit) {
          originServiceName = this.pmService.service_name
        }
        this.buildConfig = response
        if (originServiceName) {
          this.$set(this.buildConfig, 'service_name', originServiceName)
        }
        if (!this.buildConfig.timeout) {
          this.$set(this.buildConfig, time_out, 0)
        }
        if (this.buildConfig.post_build.docker_build) {
          this.docker_enabled = true
        } else {
          this.docker_enabled = false
        }
        if (this.buildConfig.post_build.file_archive) {
          this.binary_enabled = true
        } else {
          this.binary_enabled = false
        }
        if (this.buildConfig.sshs && (this.buildConfig.sshs.length !== 0 || this.buildConfig.sshs === [])) {
          this.useSshKey = true
        } else {
          this.useSshKey = false
        }
      })
    },
    initEnvConfig () {
      this.pmService.env_configs = [{
        env_name: 'dev',
        host_ids: []
      }, {
        env_name: 'qa',
        host_ids: []
      }]
    },
    createHost () {
      this.dialogHostCreateFormVisible = true
    },
    addHostOperation () {
      this.$refs['add-host'].validate(valid => {
        if (valid) {
          const payload = this.host
          payload.private_key = window.btoa(payload.private_key)
          this.dialogHostCreateFormVisible = false
          createHostAPI(payload).then((res) => {
            this.$refs['add-host'].resetFields()
            getHostListAPI().then((res) => {
              this.allHost = res
            })
            this.$message({
              type: 'success',
              message: '新增主机信息成功'
            })
          })
        } else {
          return false
        }
      })
    },
    changeAdvancedShow (index) {
      if (this.showCheckStatusAdvanced[index]) {
        this.$set(this.showCheckStatusAdvanced, index, false)
      } else {
        this.$set(this.showCheckStatusAdvanced, index, true)
      }
    },
    deleteCheck (index) {
      this.pmService.health_checks.splice(index, 1)
    },
    addCheck (index) {
      this.$refs.healthCheck.validate((valid) => {
        if (valid) {
          this.pmService.health_checks.push({
            protocol: 'http',
            path: '',
            time_out: null,
            interval: null,
            healthy_threshold: null,
            unhealthy_threshold: null
          })
        } else {
          return false
        }
      })
    },
    validateHealthCheckTimeout (rule, value, callback) {
      const reg = /^[0-9]+.?[0-9]*/
      if (value) {
        if (!reg.test(value)) {
          callback(new Error('时间应为数字'))
        } else {
          if (value >= 2 && value <= 60) {
            callback()
          } else {
            callback(new Error('请输入正确的时间范围（2-60）'))
          }
        }
      } else {
        callback(new Error('请输入正确的时间范围（2-60）'))
      }
    },
    validateHealthCheckThreshold (rule, value, callback) {
      const reg = /^[0-9]+.?[0-9]*/
      if (value) {
        if (!reg.test(value)) {
          callback(new Error('阈值应为数字'))
        } else {
          if (value >= 2 && value <= 60) {
            callback()
          } else {
            callback(new Error('请输入正确的阈值范围（2-10）'))
          }
        }
      } else if (value === 0) {
        callback(new Error('请输入正确的阈值范围（2-10）'))
      } else {
        callback()
      }
    },
    validateHealthCheckInterval (rule, value, callback) {
      const reg = /^[0-9]+.?[0-9]*/
      if (value) {
        if (!reg.test(value)) {
          callback(new Error('时间应为数字'))
        } else {
          if (value >= 2 && value <= 60) {
            callback()
          } else {
            callback(new Error('请输入正确的时间范围'))
          }
        }
      } else if (value === 0) {
        callback(new Error('请输入正确的时间范围'))
      } else {
        callback()
      }
    },
    validateHealthCheckPort (rule, value, callback) {
      const reg = /^[0-9]+.?[0-9]*/
      if (value) {
        if (!reg.test(value)) {
          callback(new Error('端口应为数字'))
        } else {
          if (value >= 1 && value <= 65535) {
            callback()
          } else {
            callback(new Error('请输入正确的端口范围'))
          }
        }
      } else if (value === 0) {
        callback(new Error('请输入正确的端口范围'))
      } else {
        callback()
      }
    },
    addEnvConfig (index) {
      this.$refs.envConfig.validate((valid) => {
        if (valid) {
          this.pmService.env_configs.push({
            env_name: '',
            host_ids: []
          })
        } else {
          return false
        }
      })
    },
    addFirstEnvConfig () {
      this.pmService.env_configs.push({
        env_name: '',
        host_ids: []
      })
    },
    deleteEnvConfig (index) {
      this.pmService.env_configs.splice(index, 1)
    },
    checkEnvConfig (val) {
      if (val) {
        if (this.pmService.env_configs.length === 0 || this.pmService.env_configs[0].host_ids.length === 0) {
          this.$message({ type: 'error', message: '请先为部署环境关联主机，再配置探活' })
          this.check_status_enabled = false
        }
      }
    },
    addFirstCacheDir () {
      if (!this.buildConfig.caches || this.buildConfig.caches.length === 0) {
        this.$set(this.buildConfig, 'caches', [])
        this.buildConfig.caches.push('')
      }
    },
    addCacheDir (index) {
      this.$refs.cacheDir.validate((valid) => {
        if (valid) {
          this.buildConfig.caches.push('')
        } else {
          return false
        }
      })
    },
    deleteCacheDir (index) {
      this.buildConfig.caches.splice(index, 1)
    },
    addBuildApp (index) {
      this.$refs.buildApp.validate((valid) => {
        if (valid) {
          this.buildConfig.pre_build.installs.push({
            name: '',
            version: '',
            id: ''
          })
        } else {
          return false
        }
      })
    },
    addFirstBuildApp () {
      this.buildConfig.pre_build.installs.push({
        name: '',
        version: '',
        id: ''
      })
    },
    deleteBuildApp (index) {
      this.buildConfig.pre_build.installs.splice(index, 1)
    },
    addBuildEnv (index) {
      this.$refs.buildEnv.validate((valid) => {
        if (valid) {
          this.buildConfig.pre_build.envs.push({
            key: '',
            value: '',
            is_credential: true
          })
        } else {
          return false
        }
      })
    },
    addFirstBuildEnv () {
      this.buildConfig.pre_build.envs.push({
        key: '',
        value: '',
        is_credential: true
      })
    },
    deleteBuildEnv (index) {
      this.buildConfig.pre_build.envs.splice(index, 1)
    },
    addExtra (command) {
      if (command === 'docker') {
        this.docker_enabled = true
        if (!this.buildConfig.post_build) {
          this.$set(this.buildConfig, 'post_build', {})
        }
        this.$set(this.buildConfig.post_build, 'docker_build', {
          work_dir: '',
          docker_file: '',
          build_args: ''
        })
      }
      if (command === 'stcov') {
        this.stcov_enabled = true
      }
      if (command === 'binary') {
        this.binary_enabled = true
        if (!this.buildConfig.post_build) {
          this.$set(this.buildConfig, 'post_build', {})
        }
        this.$set(this.buildConfig.post_build, 'file_archive', {
          file_location: ''
        })
      }
      this.$nextTick(this.$utils.scrollToBottom)
    },
    removeStcov () {
      this.stcov_enabled = false
      delete this.buildConfig.main_file
    },
    removeDocker () {
      this.docker_enabled = false
      delete this.buildConfig.post_build.docker_build
    },
    removeBinary () {
      this.binary_enabled = false
      delete this.buildConfig.post_build.file_archive
    },
    savePmService () {
      if (this.isEdit) {
        this.updatePmService()
      } else {
        this.createPmService()
      }
    },
    async createPmService () {
      const res = await this.validObj.validateAll()
      if (!res[1]) {
        return
      }
      const targets = [{
        product_name: this.projectName,
        service_name: this.buildConfig.service_name,
        service_module: this.buildConfig.service_name
      }]
      const buildConfigPayload = this.$utils.cloneObj({ targets, ...this.buildConfig })
      buildConfigPayload.product_name = this.projectName
      if (buildConfigPayload.pre_build.image_id) {
        const image = this.systems.find((item) => { return item.id === buildConfigPayload.pre_build.image_id })
        buildConfigPayload.pre_build.image_from = image.image_from
        buildConfigPayload.pre_build.build_os = image.value
      }
      const pmServicePayload =
      {
        product_name: this.projectName,
        service_name: buildConfigPayload.service_name,
        visibility: 'private',
        type: 'pm',
        build_name: buildConfigPayload.name,
        health_checks: this.check_status_enabled ? this.pmService.health_checks : [],
        env_configs: this.pmService.env_configs
      }
      const combinePayload = {
        pm_service_tmpl: pmServicePayload,
        build: buildConfigPayload
      }
      const refs = [this.$refs.addConfigForm, this.$refs.envConfig]
      if (this.check_status_enabled) {
        refs.push(this.$refs.healthCheck)
      }
      Promise.all(refs.map(r => r.validate())).then(() => {
        createPmServiceAPI(this.projectName, combinePayload).then(() => {
          if (this.changeUpdateEnvDisabled) {
            this.changeUpdateEnvDisabled()
          }
          this.$router.push({
            query: { serviceName: this.buildConfig.service_name }
          })
          this.$emit('listenCreateEvent', 'success')
          this.$message({
            type: 'success',
            message: '创建非容器服务成功'
          })
        }, () => {
          this.$emit('listenCreateEvent', 'failed')
          return false
        })
      }).catch(() => {
        const errDiv = document.querySelector('.is-error')
        errDiv.scrollIntoView({
          behavior: 'smooth',
          block: 'center'
        })
        errDiv.querySelector('input').focus()
      })
    },
    async updatePmService () {
      const res = await this.validObj.validateAll()
      if (!res[1]) {
        return
      }
      if (!this.check_status_enabled) {
        delete this.pmService.health_checks
      }
      if (!this.useSshKey) {
        this.buildConfig.sshs = []
      }
      const buildConfigPayload = this.$utils.cloneObj(this.buildConfig)
      const pmServicePayload = this.$utils.cloneObj(this.pmService)
      if (buildConfigPayload.pre_build.image_id) {
        const image = this.systems.find((item) => { return item.id === buildConfigPayload.pre_build.image_id })
        buildConfigPayload.pre_build.image_from = image.image_from
        buildConfigPayload.pre_build.build_os = image.value
      } else if (buildConfigPayload.pre_build.build_os) {
        const image = this.systems.find((item) => { return item.value === buildConfigPayload.pre_build.build_os })
        buildConfigPayload.pre_build.image_id = image.id
        buildConfigPayload.pre_build.image_from = image.image_from
      }
      buildConfigPayload.product_name = this.projectName
      pmServicePayload.build_name = buildConfigPayload.name
      const combinePayload = {
        pm_service_tmpl: pmServicePayload,
        build: buildConfigPayload
      }
      const refs = [this.$refs.addConfigForm]
      if (this.check_status_enabled) {
        refs.push(this.$refs.healthCheck)
      }
      Promise.all(refs.map(r => r.validate())).then(() => {
        updatePmServiceAPI(this.projectName, combinePayload).then(() => {
          if (this.changeUpdateEnvDisabled) {
            this.changeUpdateEnvDisabled()
          }
          this.$router.push({
            query: { serviceName: this.buildConfig.service_name }
          })
          this.$emit('listenCreateEvent', 'success')
          this.$message({
            type: 'success',
            message: '修改非容器服务成功'
          })
        }, () => {
          this.$emit('listenCreateEvent', 'failed')
          return false
        })
      }).catch(() => {
        const errDiv = document.querySelector('.is-error')
        errDiv.scrollIntoView({
          behavior: 'smooth',
          block: 'center'
        })
        errDiv.querySelector('input').focus()
      })
    },
    addNewService (obj) {
      this.buildConfig = {
        service_name: obj.service_name,
        version: 'stable',
        name: '',
        desc: '',
        repos: [],
        caches: [],
        timeout: 60,
        pre_build: {
          clean_workspace: false,
          res_req: 'low',
          build_os: 'xenial',
          image_id: '',
          image_from: '',
          installs: [],
          envs: [],
          enable_proxy: false,
          enable_gocov: false,
          parameters: []
        },
        scripts: '#!/bin/bash\nset -e',
        main_file: '',
        post_build: {
        },
        pm_deploy_scripts: "#<------------------------------------------------------------------------------->\n## 构建脚本中的变量均可使用，其他内置可用变量如下\n## ENV_NAME       环境名称，用于区分不同的集成环境，系统内置集成环境：dev，qa\n## PRIVATE_KEY    通过 SSH Agent 远程登录服务器使用的私钥 id_rsa  \n## USERNAME       通过 SSH Agent 远程登录到服务器的用户名称 \n## IP             SSH Agent 目标服务器的 IP 地址\n## 远程部署时，可以通过使用命令 `ssh -i $PRIVATE_KEY $USERNAME@$IP '自定义脚本'` 进行部署操作\n#<------------------------------------------------------------------------------->\n#!/bin/bash\nset -e",
        sshs: []
      }
      this.pmService.health_checks = [{
        protocol: 'http',
        path: '',
        time_out: null,
        interval: null,
        healthy_threshold: null,
        unhealthy_threshold: null
      }]
      this.check_status_enabled = false
      this.pmService.env_configs.forEach(item => {
        item.host_ids = []
      })
      this.removeBinary()
    },
    loadPage () {
      const projectName = this.projectName
      const orgId = this.currentOrganizationId
      getBuildConfigsAPI(this.projectName).then((res) => {
        this.builds = res
      })
      if (!this.isEdit) {
        listProductAPI('', projectName).then(res => {
          res.forEach(element => {
            if (element.product_name === this.projectName) {
              this.pmService.env_configs.push({
                env_name: element.env_name,
                host_ids: []
              })
            }
          })
        })
      }

      getAllAppsAPI().then((response) => {
        const apps = this.$utils.sortVersion(response, 'name', 'asc')
        this.allApps = apps.map((app, index) => {
          return { name: app.name, version: app.version, id: app.name + app.version }
        })
      })
      getCodeSourceAPI(orgId).then((response) => {
        this.allCodeHosts = response
      })
      getImgListAPI().then((response) => {
        this.systems = response
        if (!this.isEdit) {
          this.buildConfig.pre_build.image_id = this.systems[0].id
        }
      })
      getHostListAPI().then((res) => {
        this.allHost = res
      })
    }
  },
  watch: {
    'buildConfig.service_name': {
      handler (val, old_val) {
        if (!this.isEdit && val) {
          this.buildConfig.name = val + '-build'
        }
      }
    },
    async serviceName (value) {
      if (value) {
        this.loading = true
        const pmServiceName = this.serviceName
        const projectName = this.projectName
        const env_configs = []
        const envNameList = []
        const resList = await listProductAPI('', projectName).catch(error => console.log(error))
        if (resList) {
          resList.forEach(element => {
            if (element.product_name === this.projectName) {
              envNameList.push(
                element.env_name
              )
            }
          })
        }
        this.envNameList = envNameList
        const res = await serviceTemplateAPI(pmServiceName, 'pm', projectName).catch(error => console.log(error))
        if (res) {
          this.pmService = res
          if (this.pmService.health_checks && this.pmService.health_checks.length > 0) {
            this.check_status_enabled = true
          } else if (!this.pmService.health_checks) {
            this.check_status_enabled = false
            this.$set(this.pmService, 'health_checks', [{
              protocol: 'http',
              path: '',
              time_out: null,
              interval: null,
              healthy_threshold: null,
              unhealthy_threshold: null
            }])
          }
          if (this.pmService.build_name) {
            this.syncBuildConfig(this.pmService.build_name, this.buildConfigVersion, projectName)
          } else {
            this.$set(this.buildConfig, 'service_name', this.pmService.service_name)
          }
          if (res.env_configs && res.env_configs.length > 0) {
            this.pmService.env_configs.forEach(confItem => {
              if (envNameList.indexOf(confItem.env_name) === -1) {
                this.$set(confItem, 'showDelete', true)
              }
            })
            envNameList.forEach(envItem => {
              if (!(this.pmService.env_configs.filter(e => e.env_name === envItem).length > 0)) {
                env_configs.push({
                  env_name: envItem,
                  host_ids: []
                })
              }
            })
          } else {
            envNameList.forEach(envItem => {
              env_configs.push({
                env_name: envItem,
                host_ids: []
              })
            })
            this.$set(this.pmService, 'env_configs', env_configs)
          }
          this.loading = false
        }
      };
    }
  },

  computed: {
    projectName () {
      return this.$route.params.project_name
    },
    buildConfigVersion () {
      return 'stable'
    },
    currentOrganizationId () {
      return this.$store.state.login.userinfo.organization.id
    },
    isOnboarding () {
      return (!!this.$route.path.includes(`/v1/projects/create/${this.projectName}/not_k8s/config`))
    },
    useWorkspaceCache: {
      get () {
        return !this.buildConfig.pre_build.clean_workspace
      },
      set (val) {
        this.buildConfig.pre_build.clean_workspace = !val
      }
    }
  },
  created () {
    this.loadPage()
  },
  components: {
    editor: aceEditor,
    Resize
  }
}
</script>
<style lang="less" scoped>
@import url("~@assets/css/common/scroll-bar.less");

.el-input-group {
  vertical-align: middle;
}

.not-k8s-config-container {
  width: calc(~"100% - 250px");
  padding: 45px 15px 30px 15px;
  overflow-x: hidden;
  overflow-y: auto;
  font-size: 13px;
  background: #fff;

  .divider {
    width: 100%;
    height: 1px;
    margin: 5px 0 15px 0;
    background-color: #dfe0e6;

    &.item {
      width: 30%;
    }
  }

  .project-name {
    color: #1989fa;
  }

  .breadcrumb {
    .el-breadcrumb {
      font-size: 16px;
      line-height: 1.35;

      .el-breadcrumb__item__inner a:hover,
      .el-breadcrumb__item__inner:hover {
        color: #1989fa;
        cursor: pointer;
      }
    }
  }

  .anchor-container {
    position: absolute;
    right: 10px;
    margin-top: 50px;
  }

  .section {
    margin-bottom: 15px;
  }

  .el-form {
    .item-title {
      font-size: 15px;
    }

    .variable {
      color: #409eff;
      font-size: 13px;
      cursor: pointer;
    }
  }

  .form-style1 {
    .el-form-item {
      margin-bottom: 0;
    }
  }

  .app-operation {
    .el-button + .el-button {
      margin-left: 0;
    }
  }

  .operation-container {
    margin: 20px 0;

    .text {
      margin-right: 25px;
      color: #8d9199;
    }
  }

  .extra-build-container {
    width: calc(~"100% - 120px");
  }

  .healthcheck-item {
    margin-bottom: 15px;
  }

  .health-check-card {
    width: calc(~"100% - 120px");
    margin-bottom: 4px;

    .el-card__header {
      padding: none;
      border-bottom: none;

      .delete-btn {
        float: right;
        margin: 10px;
      }
    }
  }

  .deploy-script {
    width: calc(~"100% - 120px");
    margin-top: 10px;
    margin-bottom: 10px;

    .ace_editor.ace-xcode {
      &:hover {
        .scrollBar();
      }
    }
  }

  .deploy-method {
    line-height: 28px;
  }

  .add-check {
    margin-top: 10px;
  }

  .env-config {
    .env-name {
      color: #606266;
    }

    margin-top: 5px;

    .el-form-item {
      margin-bottom: 10px !important;
    }
  }

  .dialog-style {
    .el-dialog__body {
      padding: 0 20px;
    }

    .el-form-item {
      margin-bottom: 15px;
    }
  }
}
</style>
