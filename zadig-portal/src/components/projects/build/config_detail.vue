<template>
  <div class="build-config-container">
    <div class="jenkins"
         v-show="source === 'jenkins'">
      <div class="section">
        <el-form ref="jenkinsForm"
                 :model="jenkinsBuild"
                 label-position="left"
                 label-width="100px">
          <el-row>
            <el-col :span="8">
              <el-form-item label="构建来源"
                            :rules="[{ required: true, message: '构建来源不能为空' }]">
                <el-select style="width:100%"
                           v-model="source"
                           size="small"
                           :disabled="!isCreate"
                           value-key="key"
                           filterable>
                  <el-option v-for="(item,index) in orginOptions"
                             :key="index"
                             :label="item.label"
                             :value="item.value">
                  </el-option>
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="10">
              <el-form-item style="margin-left: 20px"
                            label="构建超时">
                <el-input-number size="mini"
                                 :min="1"
                                 v-model="jenkinsBuild.timeout"></el-input-number>
                <span>分钟</span>
              </el-form-item>
            </el-col>
          </el-row>
          <el-row>
            <el-col :span="8">
              <el-form-item label="构建名称"
                            prop="name"
                            :rules="[{ required: true, message: '构建名称不能为空' }]">
                <el-input v-model="jenkinsBuild.name"
                          placeholder="构建名称"
                          autofocus
                          size="small"
                          :disabled="!isCreate"
                          auto-complete="off"></el-input>
              </el-form-item>
            </el-col>
          </el-row>
          <el-row>
            <el-col :span="8">
              <el-form-item label="构建服务">
                <el-select style="width:100%"
                           v-model="jenkinsBuild.targets"
                           multiple
                           size="small"
                           value-key="key"
                           filterable>
                  <el-option v-for="(service,index) in serviceTargets"
                             :key="index"
                             :label="`${service.service_module}(${service.service_name})`"
                             :value="service">
                  </el-option>
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>
          <el-row>
            <el-col :span="8">
              <el-form-item label="jenkins job"
                            prop="jenkins_build.job_name"
                            :rules="[{ required: true, trigger: 'change', message: 'jobs不能为空' }]">
                <el-select style="width:100%"
                           v-model="jenkinsBuild.jenkins_build.job_name"
                           size="small"
                           value-key="key"
                           @change="changeJobName"
                           filterable>
                  <el-option v-for="(item,index) in jenkinsJobList"
                             :key="index"
                             :label="item"
                             :value="item">
                  </el-option>
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>
          <span class="item-title">构建参数</span>
          <div class="divider item"></div>
          <el-row v-for="(item, index) in jenkinsBuild.jenkins_build.jenkins_build_params"
                  :key="item.name">
            <el-col :span="8">
              <el-form-item label-width="140px"
                            :label="item.name"
                            class="form-item">
                <el-input size="medium"
                          v-model="item.value"
                          placeholder="请输入值"></el-input>
              </el-form-item>
            </el-col>
          </el-row>
        </el-form>
      </div>
    </div>
    <div class="zadig"
         v-show="source === 'zadig'">
      <div class="section">
        <el-form ref="addConfigForm"
                 :model="buildConfig"
                 :rules="createRules"
                 label-position="left"
                 label-width="80px">
          <el-row>
            <el-col :span="8">
              <el-form-item label="构建来源">
                <el-select style="width:100%"
                           v-model="source"
                           size="small"
                           value-key="key"
                           :disabled="!isCreate"
                           filterable>
                  <el-option v-for="(item,index) in orginOptions"
                             :key="index"
                             :label="item.label"
                             :value="item.value">
                  </el-option>
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="10">
              <el-form-item style="margin-left: 20px"
                            label="构建超时">
                <el-input-number size="mini"
                                 :min="1"
                                 v-model="buildConfig.timeout"></el-input-number>
                <span>分钟</span>
              </el-form-item>
            </el-col>
          </el-row>
          <el-row>
            <el-col :span="8">
              <el-form-item label="构建名称"
                            prop="name">
                <el-input v-model="buildConfig.name"
                          placeholder="构建名称"
                          autofocus
                          size="small"
                          :disabled="!isCreate"
                          auto-complete="off"></el-input>
              </el-form-item>
            </el-col>
          </el-row>
          <el-row>
            <el-col :span="8">
              <el-form-item label="构建服务">
                <el-select style="width:100%"
                           v-model="buildConfig.targets"
                           multiple
                           size="small"
                           value-key="key"
                           filterable>
                  <el-option v-for="(service,index) in serviceTargets"
                             :key="index"
                             :label="`${service.service_module}(${service.service_name})`"
                             :value="service">
                  </el-option>
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>

          <span class="item-title">构建环境</span>
          <div class="divider item"></div>
          <el-row :gutter="20">
            <el-col :span="6">
              <el-form-item label="操作系统">
                <el-select size="small"
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
            <el-col :span="6">
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
                     style="padding:0"
                     @click="addFirstBuildApp()"
                     type="text">新增</el-button>
          <div class="divider item"></div>
          <el-row v-for="(app,build_app_index) in buildConfig.pre_build.installs"
                  :key="build_app_index">
            <el-col :span="5">
              <el-form-item :prop="'pre_build.installs.' + build_app_index + '.name'"
                            :rules="{required: true, message: '应用名不能为空', trigger: 'blur'}">
                <el-select style="width:100%"
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
            <el-col :span="5">
              <el-form-item>
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
                     showFirstLine
                     showDivider></repo-select>
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
                     style="padding:0"
                     @click="addFirstBuildEnv()"
                     type="text">新增</el-button>
          <div class="divider item"></div>
          <el-row v-for="(app,build_env_index) in buildConfig.pre_build.envs"
                  :key="build_env_index">
            <el-col :span="4">
              <el-form-item :prop="'pre_build.envs.' + build_env_index + '.key'"
                            :rules="{required: true, message: '键 不能为空', trigger: 'blur'}">
                <el-input placeholder="键"
                          v-model="buildConfig.pre_build.envs[build_env_index].key"
                          size="small">
                </el-input>
              </el-form-item>
            </el-col>
            <el-col :span="4">
              <el-form-item :prop="'pre_build.envs.' + build_env_index + '.value'"
                            :rules="{required: true, message: '值 不能为空', trigger: 'blur'}">
                <el-input placeholder="值"
                          v-model="buildConfig.pre_build.envs[build_env_index].value"
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
              <el-form-item>
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
                <el-switch v-model="useWorkspaceCache"
                           active-color="#409EFF">
                </el-switch>
              </el-form-item>
            </el-col>
          </el-row>
          <template>
            <el-row>
              <el-col :span="4">
                <el-form-item label="缓存自定义目录">
                  <el-button v-if="!this.buildConfig.caches||this.buildConfig.caches.length ===0"
                             style="padding:0"
                             @click="addFirstCacheDir()"
                             type="text">新增</el-button>
                </el-form-item>
              </el-col>
            </el-row>
            <el-row v-for="(dir,index) in buildConfig.caches"
                    :key="index">
              <el-col :span="8">
                <el-form-item :label="index===0?'':''">
                  <el-input v-model="buildConfig.caches[index]"
                            style="width:100%"
                            size="small">
                    <template slot="prepend">$WORKSPACE/</template>
                  </el-input>
                </el-form-item>
              </el-col>
              <el-col :span="6">
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
      <div class="section">
        <el-form ref="buildScript"
                 :model="buildConfig"
                 label-position="left"
                 label-width="80px">
          <span class="item-title">构建脚本</span>

          <el-tooltip effect="dark"
                      placement="top-start">
            <div slot="content">
              当前可用环境变量如下，可在构建脚本中进行引用<br>
              $WORKSPACE 工作目录<br>
              $IMAGE
              &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;输出镜像名称<br>
              $SERVICE&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;构建的服务名称<br>
              $PKG_FILE &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; 构建出的 tar 包名称<br>
              $ENV_NAME &nbsp;&nbsp;&nbsp; 执行的环境名称 <br>
              $BUILD_URL &nbsp;&nbsp;&nbsp; 构建任务的 URL<br>
              &lt;REPONAME&gt;_PR 构建过程中指定代码仓库使用的pull request信息<br>
              &lt;REPONAME&gt;_BRANCH 构建过程中指定代码仓库使用的分支信息<br>
              &lt;REPONAME&gt;_TAG 构建过程中指定代码仓库使用tag信息
            </div>
            <span class="variable">变量</span>
          </el-tooltip>
          <div class="divider item"></div>
          <el-row>
            <el-col class="deploy-script"
                    :span="24">
              <editor v-model="buildConfig.scripts"
                      lang="sh"
                      theme="xcode"
                      :options="editorOption"
                      width="100%"
                      height="200px"></editor>
            </el-col>
          </el-row>
        </el-form>
        <el-form v-if="docker_enabled"
                 :model="buildConfig.post_build.docker_build"
                 :rules="docker_rules"
                 ref="docker_build"
                 label-width="220px"
                 class="docker label-at-left input-width-middle">

          <div class="dashed-container">
            <span class="title">镜像构建
              <el-button type="text"
                         @click="removeDocker"
                         icon="el-icon-delete"></el-button>
            </span>
            <div v-if="allRegistry.length === 0"
                 class="registry-alert">
              <el-alert title="私有镜像仓库未集成，请联系系统管理员前往系统设置-> Registry 管理  进行集成"
                        type="warning">
              </el-alert>
            </div>
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
                 label-width="220px"
                 class="stcov label-at-left">

          <div class="dashed-container">
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
        <el-form v-if="post_script_enabled"
                 :model="buildConfig.post_build"
                 ref="script"
                 label-width="220px"
                 class="stcov label-at-left">
          <div class="dashed-container">
            <span class="title">Shell 脚本
              <el-button type="text"
                         @click="removeScript"
                         icon="el-icon-delete"></el-button>
            </span>
            <div class="divider item"></div>
            <el-row>
              <el-col :span="24">
                <editor v-model="buildConfig.post_build.scripts"
                        lang="sh"
                        theme="xcode"
                        :options="editorOption"
                        width="100%"
                        height="300px"></editor>
              </el-col>
            </el-row>
          </div>
          <div class="divider">
          </div>
        </el-form>
        <div>
          <el-dropdown @command="addExtra">
            <el-button size="small">
              新增构建步骤<i class="el-icon-arrow-down el-icon--right"></i>
            </el-button>
            <el-dropdown-menu slot="dropdown">
              <el-dropdown-item command="docker"
                                :disabled="docker_enabled">镜像构建</el-dropdown-item>
              <el-dropdown-item command="binary"
                                :disabled="binary_enabled">二进制包归档</el-dropdown-item>
              <el-dropdown-item command="script"
                                :disabled="post_script_enabled">Shell 脚本</el-dropdown-item>
            </el-dropdown-menu>
          </el-dropdown>
        </div>
      </div>
    </div>
    <footer class="create-footer">
      <el-row>
        <el-col :span="12">
          <div class="grid-content">
            <div class="description">
              <el-tag type="primary">填写相关信息，然后点击保存</el-tag>
            </div>
          </div>
        </el-col>
        <el-col :span="12">
          <div class="grid-content button-container">
            <router-link :to="`/v1/projects/detail/${this.projectName}/builds`">
              <el-button style="margin-right:15px"
                         class="btn-primary"
                         type="primary">取消</el-button>
            </router-link>
            <el-button v-if="!isCreate"
                       class="btn-primary"
                       @click="saveBuildConfig"
                       type="primary">保存</el-button>
            <el-button v-else
                       class="btn-primary"
                       @click="createBuildConfig"
                       type="primary">保存</el-button>
          </div>
        </el-col>
      </el-row>
    </footer>
  </div>
</template>
<script>
import { getBuildConfigDetailAPI, getAllAppsAPI, getImgListAPI, getCodeSourceAPI, createBuildConfigAPI, updateBuildConfigAPI, getServiceTargetsAPI, getRegistryWhenBuildAPI, queryJenkinsJob, queryJenkinsParams } from '@api';
import aceEditor from 'vue2-ace-bind';
import bus from '@utils/event_bus';
import ValidateSubmit from '@utils/validate_async';
let validateBuildConfigName = (rule, value, callback) => {
  if (value === '') {
    callback(new Error('请输入构建名称'));
  } else {
    if (!/^[a-z0-9-]+$/.test(value)) {
      callback(new Error('名称只支持小写字母和数字，特殊字符只支持中划线'));
    } else {
      callback();
    }
  }
};
export default {
  data() {
    return {
      source: 'zadig',
      orginOptions: [{
        value: 'zadig',
        label: 'Zadig 构建'
      },
      {
        value: 'jenkins',
        label: 'Jenkins 构建'
      }],
      jenkinsJobList: [],
      jenkinsBuild: {
        "version": "stable",                       // mandatory, [release|test]
        "name": "",                // mandatory
        "desc": "",                   // optional
        targets: [],
        "timeout": 60,
        "jenkins_build": {
          "job_name": '',
          "jenkins_build_params": []
        },
        "pre_build": {
          "res_req": "low",
        }                    // mandatory

      },
      buildConfig: {
        "version": "stable",                       // mandatory, [release|test]
        "name": "",                // mandatory
        "desc": "",                   // optional
        "repos": [],
        "timeout": 60,                                 // optional
        "pre_build": {                              // mandatory
          "clean_workspace": false,
          "res_req": "low",        // optional 
          "build_os": "xenial",
          "image_id": "",                  // mandatory [precise|trusty|xenial|test]
          "installs": [],                        // optional 
          // {name: '',version: '',id: ''}
          "envs": [],                            // optional 
          "enable_proxy": false,                  // optional, default is false
          "enable_gocov": false,                  // optional, default is false
          "parameters": []                        // optional 
        },
        "scripts": '#!/bin/bash\nset -e',
        "main_file": "",
        "post_build": {          // optional
        }
      },
      editorOption: {
        enableEmmet: true,
        showLineNumbers: true,
        showFoldWidgets: true,
        showGutter: false,
        displayIndentGuides: false,
        showPrintMargin: false
      },
      stcov_enabled: false,
      docker_enabled: false,
      binary_enabled: false,
      post_script_enabled: false,
      allApps: [],
      allRegistry: [],
      serviceTargets: [],
      allCodeHosts: [],
      showBuildAdvancedSetting: {},
      createRules: {
        name: [
          {
            type: 'string',
            required: true,
            validator: validateBuildConfigName,
            trigger: 'blur'
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
      stcov_rules: {
        main_file: [
          {
            type: 'string',
            message: '请填写main文件路径',
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
      systems: [],
      validObj: new ValidateSubmit(),
    }
  },
  methods: {
    clearSelectVersion(index) {
      this.buildConfig.pre_build.installs[index].version = '';
    },
    addFirstCacheDir() {
      if (!this.buildConfig.caches || this.buildConfig.caches.length === 0) {
        this.$set(this.buildConfig, 'caches', []);
        this.buildConfig.caches.push('');
      }
    },
    addCacheDir(index) {
      this.$refs['cacheDir'].validate((valid) => {
        if (valid) {
          this.buildConfig.caches.push('');
        } else {
          return false;
        }
      });
    },
    deleteCacheDir(index) {
      this.buildConfig.caches.splice(index, 1);
    },
    addBuildApp(index) {
      this.$refs['buildApp'].validate((valid) => {
        if (valid) {
          this.buildConfig.pre_build.installs.push({
            name: '',
            version: '',
            id: ''
          });
        } else {
          return false;
        }
      });
    },
    addFirstBuildApp() {
      this.buildConfig.pre_build.installs.push({
        name: '',
        version: '',
        id: ''
      });
    },
    deleteBuildApp(index) {
      this.buildConfig.pre_build.installs.splice(index, 1);
    },
    addBuildEnv(index) {
      this.$refs['buildEnv'].validate((valid) => {
        if (valid) {
          this.buildConfig.pre_build.envs.push({
            key: '',
            value: '',
            is_credential: true
          });
        } else {
          return false;
        }
      });
    },
    addFirstBuildEnv() {
      this.buildConfig.pre_build.envs.push({
        key: '',
        value: '',
        is_credential: true
      });
    },
    deleteBuildEnv(index) {
      this.buildConfig.pre_build.envs.splice(index, 1);
    },
    addExtra(command) {
      if (command === 'docker') {
        this.docker_enabled = true;
        if (!this.buildConfig.post_build) {
          this.$set(this.buildConfig, 'post_build', {});
        }
        this.$set(this.buildConfig.post_build, 'docker_build', {
          "work_dir": "",     // mandatory, default is .
          "docker_file": "",   // mandatory, default is Dockerfile
          "build_args": ""        // optional
        });
      }
      if (command === 'stcov') {
        this.stcov_enabled = true;
      }
      if (command === 'binary') {
        this.binary_enabled = true;
        if (!this.buildConfig.post_build) {
          this.$set(this.buildConfig, 'post_build', {});
        }
        this.$set(this.buildConfig.post_build, 'file_archive', {
          "file_location": "",
        });
      }
      if (command === 'script') {
        this.post_script_enabled = true;
        if (!this.buildConfig.post_build) {
          this.$set(this.buildConfig, 'post_build', {});
        }
        this.$set(this.buildConfig.post_build, 'scripts', '#!/bin/bash\nset -e');
      }
      this.$nextTick(this.$utils.scrollToBottom);
    },
    removeStcov() {
      this.stcov_enabled = false;
      delete this.buildConfig.main_file;
    },
    removeDocker() {
      this.docker_enabled = false;
      delete this.buildConfig.post_build.docker_build;
    },
    removeBinary() {
      this.binary_enabled = false;
      delete this.buildConfig.post_build.file_archive;
    },
    removeScript() {
      this.post_script_enabled = false;
      delete this.buildConfig.post_build.scripts;
    },
    async createBuildConfig() {
      let payload = null
      let formName = null
      if (this.source === 'zadig') {
        let res = await this.validObj.validateAll();
        if (!res[1]) {
          return
        }
        formName = 'addConfigForm'
        payload = this.$utils.cloneObj(this.buildConfig);
        payload.repos.map(repo => {
          this.allCodeHosts.map(codehost => {
            if (repo.codehost_id === codehost.id) {
              repo.source = codehost.type;
            }
          });
        });
        if (payload.pre_build.image_id) {
          const image = this.systems.find((item) => { return item.id === payload.pre_build.image_id });
          payload.pre_build.image_from = image.image_from;
          payload.pre_build.build_os = image.value;
        }
      } else {
        formName = 'jenkinsForm'
        payload = this.$utils.cloneObj(this.jenkinsBuild);
      }
      payload.source = this.source
      payload.product_name = this.projectName;
      this.$refs[formName].validate(valid => {
        if (valid) {
          createBuildConfigAPI(payload).then(() => {
            this.$router.push(`/v1/projects/detail/${this.projectName}/builds`);
            this.$message({
              type: 'success',
              message: '新建构建配置成功'
            });
          });
        } else {
          return false;
        }
      });
    },
    async saveBuildConfig() {
      let payload = null
      if (this.source === 'zadig') {
        let res = await this.validObj.validateAll();
        if (!res[1]) {
          return;
        }
        payload = this.$utils.cloneObj(this.buildConfig);
        payload.repos.map(repo => {
          this.allCodeHosts.map(codehost => {
            if (repo.codehost_id === codehost.id) {
              repo.source = codehost.type;
            }
          });
        });
        if (payload.pre_build.image_id) {
          const image = this.systems.find((item) => { return item.id === payload.pre_build.image_id });
          payload.pre_build.image_from = image.image_from;
          payload.pre_build.build_os = image.value;
        }
        else if (payload.pre_build.build_os) {
          const image = this.systems.find((item) => { return item.value === payload.pre_build.build_os });
          payload.pre_build.image_id = image.id;
          payload.pre_build.image_from = image.image_from;
        }
      } else {
        payload = this.$utils.cloneObj(this.jenkinsBuild);
      }
      payload.source = this.source
      payload.productName = this.projectName;
      updateBuildConfigAPI(payload).then((response) => {
        this.$router.push(`/v1/projects/detail/${this.projectName}/builds`);
        this.$message({
          message: '保存成功',
          type: 'success'
        });
      });
    },
    async getJenkinsJob() {
      let res = await queryJenkinsJob().catch(error => console.log(error))
      if (res) {
        this.jenkinsJobList = res
      }
    },
    async changeJobName(value) {
      let res = await queryJenkinsParams(value).catch(error => console.log(error))
      if (res) {
        this.jenkinsBuild.jenkins_build.jenkins_build_params = res
      }
    },
    loadPage() {
      const projectName = this.projectName;
      const orgId = this.currentOrganizationId;
      if (!this.isCreate) {
        getBuildConfigDetailAPI(this.buildConfigName, this.buildConfigVersion, this.projectName).then((response) => {
          response.pre_build.installs.forEach(element => {
            element.id = element.name + element.version;
          });
          response.targets.forEach(t => {
            t.key = t.service_name + '/' + t.service_module;
          });
          this.buildConfig = response;
          if (this.buildConfig.source) {
            this.source = this.buildConfig.source
            if (this.source === 'jenkins') {
              this.jenkinsBuild = response
            }
          }
          if (!this.buildConfig.timeout) {
            this.$set(this.buildConfig, 'timeout', 60)
          }
          if (this.buildConfig.post_build.docker_build) {
            this.docker_enabled = true;
          }
          if (this.buildConfig.post_build.file_archive) {
            this.binary_enabled = true;
          }
          if (this.buildConfig.post_build.scripts) {
            this.post_script_enabled = true;
          }
        });
      }
      getAllAppsAPI().then((response) => {
        let apps = this.$utils.sortVersion(response, 'name', 'asc');
        this.allApps = apps.map((app, index) => {
          return { 'name': app.name, 'version': app.version, 'id': app.name + app.version }
        });
      });
      getCodeSourceAPI(orgId).then((response) => {
        this.allCodeHosts = response;
      });
      getServiceTargetsAPI(projectName).then((response) => {
        this.serviceTargets = [...response, ...this.buildConfig.targets].map(element => {
          element.key = element.service_name + '/' + element.service_module
          return element
        })
      });
      getImgListAPI().then((response) => {
        this.systems = response;
        if (this.isCreate) {
          this.buildConfig.pre_build.image_id = this.systems[0].id;
        }
      });
      getRegistryWhenBuildAPI().then((res) => {
        this.allRegistry = res;
      });
    }
  },
  computed: {
    buildConfigName() {
      return this.$route.params.build_name;
    },
    buildConfigVersion() {
      return this.$route.params.version;
    },
    currentOrganizationId() {
      return this.$store.state.login.userinfo.organization.id;
    },
    projectName() {
      return this.$route.params.project_name;
    },
    isCreate() {
      return this.$route.path === `/v1/projects/detail/${this.projectName}/builds/create`;
    },
    useWorkspaceCache: {
      get() {
        return !this.buildConfig.pre_build.clean_workspace
      },
      set(val) {
        this.buildConfig.pre_build.clean_workspace = !val
      }
    }
  },
  watch: {
    source(value) {
      if (value === 'jenkins') {
        this.getJenkinsJob()
      }
    }
  },
  created() {
    this.loadPage();
    if (this.isCreate) {
      bus.$emit(`set-topbar-title`, { title: '', breadcrumb: [{ title: '项目', url: '/v1/projects' }, { title: this.projectName, url: `/v1/projects/detail/${this.projectName}` }, { title: '构建', url: `/v1/projects/detail/${this.projectName}/builds` }, { title: '新建', url: '' }] });
    } else {
      bus.$emit(`set-topbar-title`, { title: '', breadcrumb: [{ title: '项目', url: '/v1/projects' }, { title: this.projectName, url: `/v1/projects/detail/${this.projectName}` }, { title: '构建', url: `/v1/projects/detail/${this.projectName}/builds` }, { title: this.buildConfigName, url: '' }] });
    }
    bus.$emit(`set-sub-sidebar-title`, {
      title: this.projectName,
      url: `/v1/projects/detail/${this.projectName}`,
      routerList: [
        { name: '工作流', url: `/v1/projects/detail/${this.projectName}/pipelines` },
        { name: '集成环境', url: `/v1/projects/detail/${this.projectName}/envs` },
        { name: '服务', url: `/v1/projects/detail/${this.projectName}/services` },
        { name: '构建', url: `/v1/projects/detail/${this.projectName}/builds` }
      ]
    });
  },
  components: {
    editor: aceEditor
  }

}
</script>
<style lang="less" scoped>
.el-input-group {
  vertical-align: middle;
}
.deploy-script {
  border: 1px solid #ccc;
  border-radius: 2px;
  margin-top: 10px;
  margin-bottom: 10px;
  width: calc(~"100% - 120px");
}
.params-dialog {
  background: #f5f5f5;
  padding: 10px;
  margin-bottom: 10px;
  display: inline-block;
  .delete-param {
    cursor: pointer;
    float: right;
    margin-top: -18px;
    font-size: 18px;
    color: #ff4949;
  }
}
.create-footer {
  position: fixed;
  box-sizing: border-box;
  right: 130px;
  width: 400px;
  border-radius: 4px;
  bottom: 0;
  padding: 10px 10px 10px 10px;
  z-index: 5;
  text-align: left;
  background-color: transparent;
  .btn-primary {
    color: #1989fa;
    background-color: rgba(25, 137, 250, 0.04);
    border-color: rgba(25, 137, 250, 0.4);
    &:hover {
      color: #fff;
      background-color: #1989fa;
      border-color: #1989fa;
    }
  }
  .grid-content {
    border-radius: 4px;
    min-height: 36px;
    .description {
      line-height: 36px;
      p {
        margin: 0;
        text-align: left;
        line-height: 36px;
        font-size: 16px;
        color: #676767;
      }
    }
    &.button-container {
      float: right;
    }
  }
}
.build-config-container {
  flex: 1;
  position: relative;
  overflow: auto;
  padding: 15px 20px;
  .divider {
    height: 1px;
    background-color: #dfe0e6;
    margin: 5px 0 15px 0;
    width: 100%;
    &.item {
      width: 30%;
    }
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
  .registry-alert {
    margin-bottom: 10px;
  }
  .section {
    margin-bottom: 15px;
    .input-width-middle {
      .el-form-item__content {
        margin-right: 200px;
      }
    }
  }
  .el-form {
    .item-title {
      font-size: 15px;
    }
    .variable {
      font-size: 13px;
      color: #409eff;
    }
  }
  .form-style1 {
    .el-form-item {
      margin-bottom: 0;
    }
  }
  .operation-container {
    margin: 20px 0;
    .text {
      color: #8d9199;
      margin-right: 25px;
    }
  }
}
</style>

