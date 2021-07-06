<template>
  <div class="function-test-detail">
    <el-form class="basic-info-form"
             :model="test"
             ref="test-form"
             :rules="rules"
             label-width="120px">
      <el-form-item prop="name"
                    label="测试名称"
                    class="fixed-width">
        <el-input :disabled="isEdit"
                  size="small"
                  v-model="test.name"></el-input>
      </el-form-item>
      <el-form-item label="描述信息"
                    class="fixed-width">
        <el-input size="small"
                  v-model="test.desc"></el-input>
      </el-form-item>
      <el-form-item label="测试超时">
        <el-input-number size="mini"
                         v-model="test.timeout">
        </el-input-number>
        <span>分钟</span>
      </el-form-item>
      <div class="divider"></div>

      <div class="title">执行环境</div>
      <el-row :gutter="20">
        <el-col :span="9">
          <el-form-item prop="pre_test.image_id"
                        label="操作系统">
            <el-select size="small"
                       v-model="test.pre_test.image_id"
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
        <el-col :span="9">
          <el-form-item prop="pre_test.res_req"
                        label="资源规格">

            <el-select size="small"
                       v-model="test.pre_test.res_req"
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
      <div class="divider">
      </div>

      <div class="repo dashed-container">
        <span class="title">应用列表 <el-button v-if="test.pre_test.installs.length===0"
                     @click="addInstall"
                     type="text">添加</el-button></span>
        <el-row v-for="(app, installIndex) in test.pre_test.installs"
                :key="installIndex"
                :gutter="10">
          <el-form :model="app"
                   :rules="install_items_rules"
                   ref="install_items_ref"
                   class="install-form"
                   label-position="top">
            <el-col :span="4">
              <el-form-item prop="name"
                            :label="installIndex===0?'名称':''">
                <el-select size="small"
                           @change="clearAppVersion(app)"
                           v-model="app.name"
                           placeholder="请选择应用"
                           filterable>
                  <el-option v-for="(arr, name) in allApps"
                             :key="name"
                             :label="name"
                             :value="name">
                  </el-option>
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="4">
              <el-form-item prop="version"
                            :label="installIndex===0?'版本':''">
                <el-select size="small"
                           v-model="app.version"
                           placeholder="请选择版本"
                           filterable>
                  <el-option v-for="(app_info, idx) in allApps[app.name]"
                             :key="idx"
                             :label="app_info.version"
                             :value="app_info.version">
                  </el-option>
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item :label="installIndex===0?'操作':''">
                <div>
                  <el-button size="small"
                             v-if="installIndex===test.pre_test.installs.length-1"
                             @click="addInstall"
                             type="primary"
                             plain>新增</el-button>
                  <el-button size="small"
                             v-if="test.pre_test.installs.length >= 1"
                             @click="removeInstall(installIndex)"
                             type="danger"
                             plain>删除</el-button>
                </div>
              </el-form-item>
            </el-col>
          </el-form>
        </el-row>
      </div>
      <div class="divider">
      </div>

      <el-form-item label-width="0px">
        <div class="repo dashed-container">
          <repo-select :config="test"
                       :validObj="validObj"
                       hidePrimary></repo-select>
        </div>
      </el-form-item>
      <div class="divider">
      </div>

      <el-form-item label-width="0px"
                    class="repo">
        <label class="title">
          <slot name="label">
            <span> 环境变量</span>
            <el-button size="small"
                       v-if="test.pre_test.envs.length===0"
                       @click="addEnv()"
                       type="text">添加</el-button>
          </slot>
        </label>
        <el-row v-for="(_env,index) in test.pre_test.envs"
                :key="index">
          <el-form :model="_env"
                   :rules="env_refs_rules"
                   ref="env_ref"
                   :inline="true"
                   class="env-form-inline">
            <el-form-item prop="key">
              <el-input size="small"
                        v-model="_env.key"
                        placeholder="key"></el-input>
            </el-form-item>
            <el-form-item prop="value">
              <el-input size="small"
                        v-model="_env.value"
                        placeholder="value"></el-input>
            </el-form-item>
            <el-form-item prop="is_credential">
              <el-checkbox v-model="_env.is_credential">
                敏感信息
                <el-tooltip effect="dark"
                            content="在日志中将被隐藏"
                            placement="top">
                  <i class="el-icon-question"></i>
                </el-tooltip>
              </el-checkbox>
            </el-form-item>
            <el-form-item>
              <el-button size="small"
                         v-if="index===test.pre_test.envs.length-1"
                         type="primary"
                         @click="addEnv(index)"
                         plain>添加</el-button>
              <el-button size="small"
                         v-if="index!==0 || index===0&&test.pre_test.envs.length>=1"
                         @click="deleteEnv(index)"
                         type="danger"
                         plain>删除</el-button>
            </el-form-item>
          </el-form>
        </el-row>
      </el-form-item>
      <div class="divider">
      </div>

      <div class="title">缓存策略</div>
      <el-form-item label-width="130px"
                    label="使用工作空间缓存">
        <el-switch v-model="useWorkspaceCache"></el-switch>
      </el-form-item>
      <div class="divider">
      </div>
      <div class="trigger">
        <el-form-item label-width="0px"
                      class="repo">
          <label class="title">
            <slot name="label">
              <span> 触发器</span>
              <el-button @click="addTrigger"
                         type="text">添加</el-button>
            </slot>
          </label>
          <test-trigger ref="trigger"
                        :projectName="projectName"
                        :testName="isEdit?name:test.name"
                        :webhook="test.hook_ctl"
                        :avaliableRepos="test.repos"></test-trigger>
        </el-form-item>
      </div>
      <div class="divider"></div>
      <div class="timer">
        <el-form-item label-width="0px"
                      class="repo">
          <label class="title">
            <slot name="label">
              <span> 定时器</span>
              <el-button @click="addTimer"
                         type="text">添加</el-button>
            </slot>
          </label>
          <test-timer ref="timer"
                      timerType="test"
                      :projectName="projectName"
                      :testName="isEdit?name:test.name"
                      :schedules="test.schedules"></test-timer>
        </el-form-item>
      </div>
      <div class="divider">
      </div>
      <label class="title">
        <slot name="label">
          <span>测试报告</span>
          <!-- <el-tooltip effect="dark"
                            placement="top">
                  <div slot="content">设置一个或者多个文件目录，构建完成后可以在工作流详情页面进行下载，通常用于测试日志等文件的导出<br /></div>
                  <i class="el-icon-question"></i>
                </el-tooltip>
                <el-button size="small"
                           v-if="test.artifact_paths.length===0"
                           @click="addArtifactPath()"
                           type="text">添加</el-button> -->
        </slot>
      </label>
      <el-form-item label-width="150px"
                    label="Junit 测试报告目录"
                    prop="test_result_path"
                    class="fixed-width">
        <el-input size="small"
                  v-model="test.test_result_path"
                  style="width: 80%;"
                  placeholder="请输入测试报告目录">
          <template slot="prepend">$WORKSPACE/</template>
        </el-input>
      </el-form-item>
      <el-form-item label-width="150px"
                    class="fixed-width">
        <template slot="label">
          <span>Html 测试报告文件</span>
          <el-tooltip effect="dark"
                      placement="top">
            <div slot="content">Html 测试报告文件将包含在工作流发送的 IM 通知内容中<br /></div>
            <i class="el-icon-question"></i>
          </el-tooltip>
        </template>
        <el-input size="small"
                  v-model="test.test_report_path"
                  style="width: 80%;"
                  placeholder="请输入测试报告文件">
          <template slot="prepend">$WORKSPACE/</template>
        </el-input>
      </el-form-item>
      <el-form-item label-width="150px"
                    class="fixed-width">
        <template slot="label">
          <span>测试文件导出路径</span>
          <el-tooltip effect="dark"
                      placement="top">
            <div slot="content">设置一个或者多个文件目录，构建完成后可以在工作流详情页面进行下载，通常用于测试日志等文件的导出<br /></div>
            <i class="el-icon-question"></i>
          </el-tooltip>
        </template>
        <div>
          <div v-for="(path,index) in test.artifact_paths"
               :key="index"
               class="show-one-line">
            <el-input size="small"
                      v-model="test.artifact_paths[index]"
                      placeholder="文件导出路径">
              <template slot="prepend">$WORKSPACE/</template>
            </el-input>
            <el-button size="small"
                       v-if="index===test.artifact_paths.length-1"
                       type="primary"
                       @click="addArtifactPath(index)"
                       plain>添加</el-button>
            <el-button size="small"
                       v-if="index!==0 || (index===0&&test.artifact_paths.length > 1)"
                       @click="deleteArtifactPath(index)"
                       type="danger"
                       plain>删除</el-button>
          </div>
        </div>
      </el-form-item>
      <div class="divider">
      </div>
      <el-form-item label-width="0px">
        <div class="script dashed-container">
          <span class="title">测试脚本</span>
          <el-tooltip effect="dark"
                      placement="top-start">
            <div slot="content">
              当前可用环境变量如下，可在构建脚本中进行引用<br />
              $WORKSPACE &nbsp; 工作目录<br />
              $LINKED_ENV &nbsp; 被测命名空间<br />
              $ENV_NAME &nbsp;&nbsp;&nbsp;&nbsp; 被测环境名称<br />
              $TEST_URL &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; 测试任务的 URL<br>
              $CI
              &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;
              值恒等于 true，表示当前环境是 CI/CD 环境<br />
              $ZADIG
              &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;
              值恒等于 true，表示在 ZADIG 系统上执行脚本<br />
            </div>
            <span class="variable">变量</span>
          </el-tooltip>
          <div class="test-container">
            <div>
              <el-row>
                <el-col :span="24">
                  <editor v-model="test.scripts"
                          lang="sh"
                          theme="xcode"
                          :options="editorOption"
                          width="100%"
                          height="200px"></editor>
                </el-col>
              </el-row>
            </div>
          </div>
        </div>
      </el-form-item>

      <div class="divider"></div>
      <footer class="create-footer">
        <el-row>
          <el-col :span="12">
            <div class="grid-content bg-purple">
              <div class="description">
                <el-tag type="primary">填写相关信息，然后点击保存</el-tag>
              </div>
            </div>
          </el-col>
          <el-col :span="12">
            <div class="grid-content button-container">
              <router-link :to="`/v1/${basePath}/detail/${projectName}/test/function`">
                <el-button class="btn-primary"
                           type="primary">取消</el-button>
              </router-link>
              <el-button class="btn-primary"
                         @click="saveTest"
                         type="primary">保存</el-button>
            </div>
          </el-col>
        </el-row>
      </footer>

    </el-form>
  </div>
</template>

<script>
import testTrigger from '@/components/common/test_trigger.vue'
import bus from '@utils/event_bus'
import ValidateSubmit from '@utils/validate_async'
import aceEditor from 'vue2-ace-bind'
import {
  getAllAppsAPI, getImgListAPI, productTemplatesAPI, getCodeSourceAPI, createTestAPI, updateTestAPI, singleTestAPI
} from '@api'
const validateTestName = (rule, value, callback) => {
  if (value === '') {
    callback(new Error('请输入测试名称'))
  } else {
    if (!/^[a-zA-Z0-9-_]+$/.test(value)) {
      callback(new Error('名称只支持字母和数字，特殊字符只支持中划线和下划线'))
    } else {
      callback()
    }
  }
}
export default {
  data () {
    return {
      productTemplates: [],
      allApps: {},
      allCodeHosts: [],
      systems: [],
      test: {
        name: '',
        product_name: '',
        desc: '',
        repos: [],
        timeout: 60,
        hook_ctl: {
          enabled: false,
          items: []
        },
        schedules: {
          enabled: false,
          items: []
        },
        pre_test: {
          enable_proxy: false,
          clean_workspace: false,
          build_os: 'xenial',
          image_from: '',
          image_id: '',
          res_req: 'low',
          installs: [
            { name: '', version: '' }
          ],
          envs: []
        },
        artifact_paths: [],
        scripts: '#!/bin/bash\nset -e',
        test_result_path: '',
        test_report_path: '',
        threshold: 0,
        post_test: {
          stcov: {
            enable: false
          }
        },
        test_type: 'function'
      },
      editorOption: {
        enableEmmet: true,
        showLineNumbers: true,
        showFoldWidgets: true,
        showGutter: false,
        displayIndentGuides: false,
        showPrintMargin: false
      },
      rules: {
        name: [
          {
            type: 'string',
            required: true,
            validator: validateTestName,
            trigger: 'change'
          }
        ],
        product_name: [
          {
            type: 'string',
            required: true,
            message: '请选择所属项目',
            trigger: 'change'
          }
        ],
        'pre_test.image_id': [
          {
            type: 'string',
            required: true,
            message: '请选择测试环境',
            trigger: 'change'
          }
        ],
        'pre_test.res_req': [
          {
            type: 'string',
            required: true,
            message: '请选择资源规格',
            trigger: 'change'
          }
        ],
        test_result_path: [
          {
            type: 'string',
            required: true,
            message: '请输入结果地址',
            trigger: 'blur'
          }
        ],
        threshold: [
          {
            type: 'number',
            required: true,
            message: '请输入测试阈值',
            trigger: 'blur'
          }
        ],
        test_type: [
          {
            type: 'string',
            required: true,
            message: '选择测试类型',
            trigger: 'blur'
          }
        ]
      },

      install_items_rules: {
        name: [
          {
            type: 'string',
            required: true,
            message: '请选择应用',
            trigger: 'change'
          }
        ],
        version: [
          {
            type: 'string',
            required: true,
            message: '请选择版本',
            trigger: 'change'
          }
        ]
      },
      env_refs_rules: {
        key: [
          {
            type: 'string',
            required: true,
            message: '请输入key',
            trigger: 'blur'
          }
        ],
        value: [
          {
            type: 'string',
            required: true,
            message: '请输入value',
            trigger: 'blur'
          }
        ]
      },
      validObj: new ValidateSubmit()
    }
  },
  props: {
    basePath: {
      request: true,
      type: String
    }
  },
  computed: {
    name () {
      return this.$route.params.test_name
    },
    isEdit () {
      return !!this.name
    },
    projectName () {
      return this.$route.params.project_name
    },
    stcovEnabled: {
      get () {
        return this.test.post_test && this.test.post_test.stcov && this.test.post_test.stcov.enable
      },
      set (val) {
        this.test.post_test = this.test.post_test || {}
        this.test.post_test.stcov = this.test.post_test.stcov || {}
        this.test.post_test.stcov.enable = val
      }
    },
    currentOrganizationId () {
      return this.$store.state.login.userinfo.organization.id
    },
    useWorkspaceCache: {
      get () {
        return !this.test.pre_test.clean_workspace
      },
      set (val) {
        this.test.pre_test.clean_workspace = !val
      }
    }
  },
  methods: {
    decodeCampaign (c) {
      return c.enabled === true
    },
    encodeCampaign (b) {
      return !!b
    },
    clearAppVersion (obj) {
      obj.version = ''
    },
    addInstall () {
      this.test.pre_test.installs.push({
        name: '',
        version: ''
      })
    },
    removeInstall (index) {
      this.test.pre_test.installs.splice(index, 1)
    },
    addTrigger () {
      this.$refs.trigger.addWebhookBtn()
    },
    addTimer () {
      this.$refs.timer.addTimerBtn()
    },
    addEnv (index) {
      this.test.pre_test.envs.push(this.makeEmptyEnv())
    },
    addArtifactPath (index) {
      this.test.artifact_paths.push('')
    },
    deleteEnv (index) {
      this.test.pre_test.envs.splice(index, 1)
    },
    deleteArtifactPath (index) {
      this.test.artifact_paths.splice(index, 1)
    },
    removeStcov () {
      this.stcovEnabled = false
    },
    addExtra (command) {
      if (command === 'stcov') {
        this.stcovEnabled = true
      }
      this.$nextTick(this.$utils.scrollToBottom)
    },
    async saveTest () {
      const refs = [this.$refs['test-form']]
        .concat(this.$refs.install_items_ref)
        .concat(this.$refs.env_ref)
        .filter(r => r)
      const res = await this.validObj.validateAll()
      if (!res[1]) {
        return
      }
      Promise.all(refs.map(r => r.validate())).then(() => {
        if (this.test.pre_test.image_id) {
          const image = this.systems.find((item) => { return item.id === this.test.pre_test.image_id })
          this.test.pre_test.image_from = image.image_from
          this.test.pre_test.build_os = image.value
        } else if (this.test.pre_test.build_os) {
          const image = this.systems.find((item) => { return item.value === this.test.pre_test.build_os })
          this.test.pre_test.image_id = image.id
          this.test.pre_test.image_from = image.image_from
        }
        (this.isEdit ? updateTestAPI : createTestAPI)(this.test).then(res => {
          this.$message({
            message: '保存成功',
            type: 'success'
          })
          this.$router.push(`/v1/${this.basePath}/detail/${this.projectName}/test/function`)
        })
      })
    },
    makeEmptyEnv () {
      return {
        key: '',
        value: '',
        is_credential: true
      }
    },

    makeMapOfArray (arr, namePropName) {
      const map = {}
      for (const obj of arr) {
        if (!map[obj[namePropName]]) {
          map[obj[namePropName]] = [obj]
        } else {
          map[obj[namePropName]].push(obj)
        }
      }
      return map
    }
  },
  created () {
    const orgId = this.currentOrganizationId
    productTemplatesAPI().then(res => {
      this.productTemplates = res
    })
    getAllAppsAPI().then(res => {
      this.allApps = this.makeMapOfArray(this.$utils.sortVersion(res, 'version', 'asc'), 'name')
    })
    getCodeSourceAPI(orgId).then((response) => {
      this.allCodeHosts = response
    })
    getImgListAPI().then((response) => {
      this.systems = response
      if (!this.isEdit) {
        this.test.pre_test.image_id = this.systems[0].id
      }
    })
    this.test.product_name = this.projectName
    const topTitle = this.basePath === 'tests'
      ? {
        title: '测试管理',
        projectUrl: `/v1/${this.basePath}/detail/${this.projectName}/test`
      }
      : {
        title: '项目',
        projectUrl: `/v1/${this.basePath}/detail/${this.projectName}`
      }
    if (this.isEdit) {
      bus.$emit(`set-topbar-title`, {
        title: '',
        breadcrumb: [
          { title: topTitle.title, url: `/v1/${this.basePath}` },
          { title: this.projectName, url: topTitle.projectUrl },
          { title: '功能测试', url: `/v1/${this.basePath}/detail/${this.projectName}/test/function` },
          { title: this.name, url: '' }
        ]
      })
      singleTestAPI(this.name, this.projectName).then(res => {
        this.test = res
        if (!res.test_report_path) {
          this.$set(this.test, 'test_report_path', '')
        }
        if (!res.artifact_paths) {
          this.$set(this.test, 'artifact_paths', [])
        }
        if (!res.hook_ctl) {
          this.$set(this.test, 'hook_ctl', {
            enabled: false,
            items: []
          })
        }
        if (!res.schedules) {
          this.$set(this.test, 'schedules', {
            enabled: false,
            items: []
          })
        }
        if (this.test.artifact_paths.length === 0) {
          this.addArtifactPath()
        }
      })
    } else {
      bus.$emit(`set-topbar-title`, {
        title: '',
        breadcrumb: [
          { title: topTitle.title, url: `/v1/${this.basePath}` },
          { title: this.projectName, url: topTitle.projectUrl },
          { title: '功能测试', url: `/v1/${this.basePath}/detail/${this.projectName}/test/function` },
          { title: '添加', url: '' }
        ]
      })
      this.addArtifactPath()
    }
  },
  components: {
    editor: aceEditor,
    testTrigger
  }
}
</script>

<style lang="less">
.function-test-detail {
  position: relative;
  flex: 1;
  padding: 15px 20px;
  overflow: auto;

  .el-row {
    margin-bottom: 20px;

    &:last-child {
      margin-bottom: 0;
    }
  }

  .basic-info-form {
    margin: 20px 0 50px;

    .el-form-item {
      margin-bottom: 20px;
    }

    .fixed-width {
      .el-input__inner {
        width: 300px;
      }
    }

    .el-form-item__label {
      text-align: left;
    }
  }

  .show-one-line {
    display: flex;
    margin-bottom: 20px;

    .el-input-group {
      width: auto;
      margin-right: 10px;
    }
  }

  .divider {
    width: 100%;
    height: 1px;
    margin: 13px 0;
    background-color: #dfe0e6;
  }

  .title {
    padding-top: 6px;
    font-size: 15px;
    line-height: 40px;
  }

  .repo {
    /* padding: 5px 8px; */
    .repo-operation-container {
      display: inline-block;

      .delete {
        color: #ff4949;
      }

      .add {
        color: #1989fa;
      }
    }
  }

  .script,
  .stcov {
    /* padding: 5px 8px; */
    .title {
      display: inline-block;
      padding-top: 6px;
      color: #606266;
      font-size: 14px;
    }

    .item-title {
      margin-left: 5px;
      color: #909399;
    }

    .test-container {
      .operation-container {
        margin-left: 40px;
        padding-top: 30px;

        .delete {
          color: #ff4949;
        }
      }

      .add-test {
        margin-top: 5px;
      }
    }
  }

  .stcov {
    .title {
      margin: 10px 0 15px 0;
    }

    .el-form-item__content {
      height: 40px;
    }
  }

  .el-form.label-at-left .el-form-item__label {
    float: left;
  }

  .el-input-group__prepend {
    padding-right: 3px;
  }

  .el-input-group__append {
    padding-left: 3px;
  }

  .el-select.appended {
    .el-input__inner {
      border-top-right-radius: 0;
      border-bottom-right-radius: 0;
    }
  }

  .select-append {
    position: relative;
    top: -2px;
    display: inline-block;
    padding: 0 20px 0 3px;
    color: #909399;
    font-size: 14px;
    line-height: 38px;
    white-space: nowrap;
    vertical-align: middle;
    background-color: #f5f7fa;
    border: 1px solid #dcdfe6;
    border-left: 0;
    border-radius: 4px;
    border-top-left-radius: 0;
    border-bottom-left-radius: 0;
  }

  .el-breadcrumb {
    font-size: 16px;
    line-height: 1.35;

    .el-breadcrumb__item__inner a:hover,
    .el-breadcrumb__item__inner:hover {
      color: #1989fa;
      cursor: pointer;
    }
  }

  .text {
    font-size: 13px;
  }

  .text-title {
    color: #8d9199;
  }

  .visibility {
    display: inline-block;
  }

  .item {
    padding: 10px 0;
    padding-left: 1px;

    .edit-icon {
      margin-left: 15px;
    }

    .icon-color {
      color: #2d2f33;
      font-size: 15px;
      cursor: pointer;

      &:hover {
        color: #1989fa;
      }
    }

    .icon-color-cancel {
      color: #ff4949;
      font-size: 15px;
      cursor: pointer;
    }

    .visibility-control {
      display: inline-block;
      width: 100px;
    }
  }

  .clearfix::before,
  .clearfix::after {
    display: table;
    content: "";
  }

  .clearfix::after {
    clear: both;
  }

  .box-card,
  .box-card-service {
    width: 100%;
    border: none;
    box-shadow: none;

    .services-container {
      .edit-operation {
        .title {
          font-size: 14px;
          line-height: 14px;
        }

        .operation-btns {
          margin-left: 25px;
        }
      }
    }
  }

  .wide-for-screen {
    width: 100%;
  }

  .el-card__header {
    padding-left: 0;
  }

  .add-extra {
    margin: 0 0 30px 0;
  }

  .artifact-form-inline {
    vertical-align: sub;
  }

  .create-footer {
    position: fixed;
    right: 130px;
    bottom: 0;
    z-index: 5;
    box-sizing: border-box;
    width: 400px;
    padding: 10px 10px 10px 10px;
    text-align: left;
    background-color: transparent;
    border-radius: 4px;

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
      min-height: 36px;
      border-radius: 4px;

      .description {
        line-height: 36px;

        p {
          margin: 0;
          color: #676767;
          font-size: 16px;
          line-height: 36px;
          text-align: left;
        }
      }

      &.button-container {
        float: right;
      }
    }
  }

  .variable {
    color: #409eff;
    font-size: 13px;
  }
}
</style>
