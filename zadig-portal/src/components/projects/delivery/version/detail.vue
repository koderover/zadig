<template>
  <div v-loading="loading"
       class="version-container">
    <el-dialog :visible.sync="exportModal.visible"
               width="65%"
               title="服务配置查看"
               class="export-dialog">
      <span v-if="exportModal.textObjects.length === 0"
            class="nothing">
        {{'没有找到数据'}}
      </span>
      <template v-else>
        <div v-for="(obj, i) of exportModal.textObjects"
             :key="obj.originalText"
             class="config-viewer">
          <div>
            <div :class="{'op-row': true, expanded: obj.expanded}">
              <el-button @click="toggleYAML(obj)"
                         type="text"
                         icon="el-icon-caret-bottom">
                {{ obj.expanded ? '收起' : '展开' }}
              </el-button>
              <el-button @click="copyYAML(obj, i)"
                         type="primary"
                         plain
                         size="small"
                         class="at-right">复制</el-button>
            </div>
            <editor v-show="obj.expanded"
                    :value="obj.readableText"
                    :options="exportModal.editorOption"
                    @init="editorInit($event, obj)"
                    lang="yaml"
                    theme="tomorrow_night"
                    width="100%"
                    height="800"></editor>
          </div>
        </div>
      </template>
    </el-dialog>
    <el-dialog :title="`版本发布 ${workflowToRun.version}`"
               :visible.sync="runWorkflowFromVersionDialogVisible"
               custom-class="run-workflow"
               width="60%">
      <div>
        <run-workflow-from-version v-if="workflowToRun.workflowName"
                                   :workflowName="workflowToRun.workflowName"
                                   :projectName="workflowToRun.projectName"
                                   :imageConfigs="imagesAndConfigs"></run-workflow-from-version>
      </div>

    </el-dialog>
    <el-tabs type="border-card">
      <el-tab-pane label="版本信息">
        <div class="el-card box-card task-process is-always-shadow">
          <div class="el-card__header">
            <div class="clearfix">
              <span>基本信息</span>
            </div>
          </div>
          <div class="el-card__body">
            <div class="text item">
              <div class="el-row">
                <div class="el-col el-col-6">
                  <div class=" item-title">版本</div>
                </div>
                <div class="el-col el-col-6">
                  <div class="item-desc">
                    {{currentVersionDetail.versionInfo.version}}
                  </div>
                </div>
                <div class="el-col el-col-6">
                  <div class=" item-title">产品</div>
                </div>
                <div class="el-col el-col-6">
                  <div class=" item-desc">
                    <span>{{currentVersionDetail.versionInfo.productName}}</span>
                  </div>
                </div>
              </div>
              <div class="el-row">
                <div class="el-col el-col-6">
                  <div class=" item-title">工作流详情</div>
                </div>
                <div class="el-col el-col-6">
                  <div class=" item-desc">
                    <span v-if="currentVersionDetail.versionInfo.taskId"
                          class="link">
                      <router-link
                                   :to="`/v1/projects/detail/${currentVersionDetail.versionInfo.productName}/pipelines/multi/${currentVersionDetail.versionInfo.workflowName}/${currentVersionDetail.versionInfo.taskId}`">
                        {{currentVersionDetail.versionInfo.workflowName + '#'
                        +currentVersionDetail.versionInfo.taskId}}</router-link>
                    </span>
                  </div>
                </div>
                <div class="el-col el-col-6">
                  <div class=" item-title">描述</div>
                </div>
                <div class="el-col el-col-6">
                  <div class="item-desc">
                    {{currentVersionDetail.versionInfo.desc}}
                  </div>
                </div>
              </div>
              <div class="el-row">
                <div class="el-col el-col-6">
                  <div class=" item-title">创建人</div>
                </div>
                <div class="el-col el-col-6">
                  <div class=" item-desc">
                    {{currentVersionDetail.versionInfo.createdBy}}
                  </div>
                </div>
                <div class="el-col el-col-6">
                  <div class=" item-title">创建时间</div>
                </div>
                <div class="el-col el-col-6">
                  <div class="item-desc">
                    {{$utils.convertTimestamp(currentVersionDetail.versionInfo.created_at)}}
                  </div>
                </div>
              </div>
              <div class="el-row">
                <div class="el-col el-col-6">
                  <div class=" item-title">标签</div>
                </div>
                <div class="el-col el-col-6">
                  <div class=" item-desc">
                    <span v-for="(label,index) in currentVersionDetail.versionInfo.labels"
                          :key="index"
                          style="margin-right: 3px;">
                      <el-tag size="small">{{label}}</el-tag>
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div class="el-card box-card task-process is-always-shadow">
          <div class="el-card__header">
            <div class="clearfix">
              <span>交付内容</span>
            </div>
          </div>
          <div v-if="jiraIssues.length > 0"
               class="el-card__body">
            <div class="text item">
              <div class="section-head">
                Jira 问题关联
              </div>
              <el-table :data="jiraIssues"
                        style="width: 100%;">

                <el-table-column label="服务名">
                  <template slot-scope="scope">
                    {{scope.row.service_name}}
                  </template>
                </el-table-column>

                <el-table-column label="关联问题">
                  <template slot-scope="scope">
                    <el-popover v-for="(issue,index) in scope.row.issues"
                                :key="index"
                                trigger="hover"
                                placement="top"
                                popper-class="issue-popper">
                      <p>报告人: {{issue.reporter?issue.reporter:'*'}}</p>
                      <p>分配给: {{issue.assignee?issue.assignee:'*'}}</p>
                      <p>优先级: {{issue.priority?issue.priority:'*'}}</p>
                      <span slot="reference"
                            class="issue-name-wrapper text-center">
                        <a :href="issue.url"
                           target="_blank">{{`${issue.key} ${issue.summary}`}}</a>
                      </span>
                    </el-popover>
                  </template>
                </el-table-column>
              </el-table>
            </div>
          </div>
          <div v-if="imagesAndConfigs.length > 0"
               class="el-card__body">
            <div class="text item">
              <div class="section-head">
                镜像和配置信息
              </div>
              <el-table :data="imagesAndConfigs"
                        style="width: 100%;">
                <el-table-column label="服务名/服务组件">
                  <template slot-scope="scope">
                    {{scope.row.serviceName}}
                  </template>
                </el-table-column>
                <el-table-column label="镜像">
                  <template slot-scope="scope">
                    <router-link :to="`/v1/delivery/artifacts?image=${scope.row.registryName}`">
                      <span class="img-link">{{scope.row.registryName}}</span>
                    </router-link>
                  </template>
                </el-table-column>
                <el-table-column label="服务配置"
                                 width="130px">
                  <template slot-scope="scope">
                    <el-button type="text"
                               @click="showConfig(scope.row.yamlContents)">查看</el-button>
                  </template>
                </el-table-column>
              </el-table>
            </div>
          </div>

          <div v-if="packages.length > 0"
               class="el-card__body">
            <div class="text item">
              <div class="section-head">
                包信息
              </div>
              <el-table :data="packages"
                        style="width: 100%;">
                <el-table-column label="服务名">
                  <template slot-scope="scope">
                    {{scope.row.serviceName}}
                  </template>
                </el-table-column>
                <el-table-column label="包文件名">
                  <template slot-scope="scope">
                    {{scope.row.packageFile}}
                  </template>
                </el-table-column>
              </el-table>
            </div>
          </div>
          <div v-if="orderedServices.length > 0"
               class="el-card__body">
            <div class="text item">
              <div class="section-head">
                服务启动顺序
              </div>
              <el-table :data="orderedServices"
                        style="width: 100%;">
                <el-table-column label="启动顺序">
                  <template slot-scope="scope">
                    {{scope.$index}}
                  </template>
                </el-table-column>
                <el-table-column label="服务名">
                  <template slot-scope="scope">
                    {{scope.row.join(' , ')}}
                  </template>
                </el-table-column>
              </el-table>
            </div>
          </div>
        </div>
      </el-tab-pane>
      <el-tab-pane label="交付清单">
        <div class="el-card box-card task-process is-always-shadow">
          <div class="el-card__header">
            <div class="clearfix"><span>交付清单</span>
            </div>
          </div>
          <div v-if="codeLists.length > 0"
               class="el-card__body">
            <div class="section-head">
              代码更新列表
            </div>
            <el-table :data="codeLists"
                      style="width: 100%;">
              <el-table-column label="名称"
                               prop="repo_name">
              </el-table-column>
              <el-table-column label="分支"
                               prop="branch">
              </el-table-column>
              <el-table-column label="Commit ID"
                               prop="commit_id">
                <template slot-scope="scope">
                  <el-tooltip :content="`在 ${scope.row.source} 上查看 Commit`"
                              placement="top"
                              effect="dark">
                    <span v-if="scope.row.commit_id"
                          class="link">
                      <a :href="`${scope.row.address}/${scope.row.repo_owner}/${scope.row.repo_name}/commit/${scope.row.commit_id}`"
                         target="_blank">{{scope.row.commit_id.substring(0, 10)}}
                      </a>
                    </span>
                  </el-tooltip>
                </template>
              </el-table-column>

            </el-table>
          </div>
          <div v-if="systemTests.length > 0"
               class="el-card__body">
            <div class="section-head">
              系统测试
            </div>
            <el-table :data="systemTests"
                      style="width: 100%;">
              <el-table-column label="名称"
                               prop="testName">
              </el-table-column>
              <el-table-column label="">
                <template slot-scope="scope">
                  {{`总数:${scope.row.tests + scope.row.skips}
                  执行:${scope.row.tests - scope.row.skips}
                  成功:${scope.row.tests -
                  scope.row.failures - scope.row.errors}
                  失败:${scope.row.failures}`}}
                </template>
              </el-table-column>
              <el-table-column label="测试报告">
                <template slot-scope="scope">
                  <span class="test-report-link">
                    <router-link
                                 :to="`/v1/projects/detail/${currentVersionDetail.versionInfo.productName}/pipelines/multi/testcase/${scope.row.workflowName}/${scope.row.taskId}/test/${scope.row.workflowName}-${scope.row.taskId}-test?is_workflow=1&service_name=${scope.row.testName}&test_type=function`">
                      查看</router-link>
                  </span>

                </template>
              </el-table-column>

            </el-table>
          </div>
          <div v-if="performanceTests.length > 0"
               class="el-card__body">
            <div class="section-head">
              性能测试
            </div>
            <el-table :data="performanceTests"
                      style="width: 100%;">
              <el-table-column label="名称"
                               prop="testName">
              </el-table-column>
              <el-table-column label="">
                <template slot-scope="scope">
                  {{`Average:${scope.row.average} Max:${scope.row.max} Min:${scope.row.min}`}}
                </template>
              </el-table-column>
              <el-table-column label="测试报告">
                <template slot-scope="scope">
                  <span class="test-report-link">
                    <router-link
                                 :to="`/v1/projects/detail/${currentVersionDetail.versionInfo.productName}/pipelines/multi/testcase/${scope.row.workflowName}/${scope.row.taskId}/test/${scope.row.workflowName}-${scope.row.taskId}-test?is_workflow=1&service_name=${scope.row.testName}&test_type=performance`">
                      查看</router-link>
                  </span>

                </template>
              </el-table-column>

            </el-table>
          </div>
          <div v-if="currentVersionDetail.securityStatsInfo && currentVersionDetail.securityStatsInfo.length > 0"
               class="el-card__body">
            <div class="section-head">
              镜像安全扫描
            </div>
            <el-table :data="currentVersionDetail.securityStatsInfo"
                      style="width: 100%;">
              <el-table-column label="镜像名称"
                               prop="imageName">
              </el-table-column>
              <el-table-column label="扫描结果">
                <template slot-scope="scope">
                  {{`漏洞总数:${scope.row.deliverySecurityStatsInfo.total}
                  最高危:${scope.row.deliverySecurityStatsInfo.critical}
                  高危:${scope.row.deliverySecurityStatsInfo.high}
                  中危:${scope.row.deliverySecurityStatsInfo.medium}
                  低危:${scope.row.deliverySecurityStatsInfo.low}`}}
                </template>
              </el-table-column>
              <el-table-column label="扫描报告">
                <template slot-scope="scope">
                  <span class="test-report-link">
                    <router-link
                                 :to="`/v1/projects/detail/${currentVersionDetail.versionInfo.productName}/pipelines/security/${currentVersionDetail.versionInfo.workflowName}/${currentVersionDetail.versionInfo.taskId}?imageId=${scope.row.imageId}`">
                      查看</router-link>
                  </span>

                </template>
              </el-table-column>

            </el-table>
          </div>
        </div>
      </el-tab-pane>
      <el-tab-pane v-if="showArtifactDeployBtn"
                   disabled>
        <span @click="runWorkflowFromVersion"
              slot="label"><i class="el-icon-upload2"></i> 版本发布</span>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script>
import { getVersionListAPI } from '@api'
import { uniqBy } from 'lodash'
import bus from '@utils/event_bus'
import aceEditor from 'vue2-ace-bind'
import runWorkflowFromVersion from './container/run_workflow.vue'
import 'brace/mode/yaml'
import 'brace/theme/xcode'
import 'brace/theme/tomorrow_night'
import 'brace/ext/searchbox'
export default {
  data () {
    return {
      workflowToRun: {
        workflowName: '',
        projectName: '',
        version: ''
      },
      loading: false,
      runWorkflowFromVersionDialogVisible: false,
      currentVersionDetail: {
        versionInfo: {},
        buildInfo: [],
        deployInfo: [],
        testInfo: [],
        securityStatsInfo: []
      },
      imagesAndConfigs: [],
      codeLists: [],
      testLists: [],
      jiraIssues: [],
      systemTests: [],
      performanceTests: [],
      orderedServices: [],
      packages: [],
      exportModal: {
        textObjects: [],
        visible: false,
        loading: false,
        editorOption: {
          showLineNumbers: true,
          showFoldWidgets: true,
          showGutter: true,
          displayIndentGuides: true,
          showPrintMargin: false,
          readOnly: true,
          tabSize: 2,
          maxLines: Infinity
        }
      },
      window: window
    }
  },
  methods: {
    runWorkflowFromVersion () {
      this.runWorkflowFromVersionDialogVisible = true
      this.workflowToRun.projectName = this.currentVersionDetail.versionInfo.productName
      this.workflowToRun.version = this.currentVersionDetail.versionInfo.version
      this.workflowToRun.workflowName = this.currentVersionDetail.versionInfo.workflowName
    },
    getVersionDetail () {
      const versionId = this.versionId
      const orgId = this.currentOrganizationId
      getVersionListAPI(orgId).then((res) => {
        const currentVersionDetail = res.find(item => item.versionInfo.id === versionId)
        this.transformData(currentVersionDetail)
        this.$set(this, 'currentVersionDetail', currentVersionDetail)
      })
    },
    transformData (current_version_info) {
      if (current_version_info.distributeInfo) {
        current_version_info.distributeInfo.forEach(distribute => {
          current_version_info.deployInfo.forEach(deploy => {
            if (distribute.serviceName === deploy.containerName) {
              distribute.yamlContents = deploy.yamlContents
            }
          })
        })
        current_version_info.distributeInfo.forEach(distribute => {
          if (distribute.distributeType !== 'file') {
            this.imagesAndConfigs.push({
              serviceName: distribute.serviceName,
              registryName: distribute.registryName,
              yamlContents: distribute.yamlContents
            })
          } else if (distribute.distributeType === 'file') {
            this.packages.push({
              serviceName: distribute.serviceName,
              packageFile: distribute.packageFile
            })
          }
        })
      }
      if (current_version_info.deployInfo) {
        current_version_info.deployInfo.forEach(artifactDeploy => {
          this.imagesAndConfigs.push({
            serviceName: artifactDeploy.serviceName,
            containerName: artifactDeploy.containerName,
            registryName: artifactDeploy.image,
            yamlContents: artifactDeploy.yamlContents,
            registryId: artifactDeploy.registry_id
          })
        })
      }

      if (current_version_info.buildInfo) {
        current_version_info.buildInfo.forEach(build => {
          // Code list
          this.codeLists = this.codeLists.concat(build.commits)
          // Jira issues
          if (build.issues.length > 0) {
            this.jiraIssues.push({
              service_name: build.serviceName,
              issues: build.issues
            })
          }
        })
      }

      if (current_version_info.testInfo) {
        current_version_info.testInfo.forEach(test => {
          // Test list
          this.testLists = this.testLists.concat(test.testReports)
        })
      }
      this.codeLists = uniqBy(this.codeLists, 'repo_name')

      this.testLists.forEach(element => {
        if (element.testSuite) {
          element.testSuite.testName = element.testName
          element.testSuite.workflowName = element.workflowName
          element.testSuite.taskId = element.taskId
          element.testSuite.testResultPath = element.testResultPath
          this.systemTests.push(element.testSuite)
        } else if (element.performanceTestSuite.length > 0) {
          element.performanceTestSuite[0].testName = element.testName
          element.performanceTestSuite[0].workflowName = element.workflowName
          element.performanceTestSuite[0].taskId = element.taskId
          element.performanceTestSuite[0].testResultPath = element.testResultPath
          this.performanceTests.push(element.performanceTestSuite[0])
        }
      })

      // orderedServices
      if (current_version_info.deployInfo.length > 0) {
        this.orderedServices = current_version_info.deployInfo[0].orderedServices
      }
    },
    showConfig (data) {
      this.exportModal.visible = true
      this.exportModal.textObjects = []
      this.exportModal.textObjects = this.$utils.mapToArray(data, 'key').map(txt => ({
        originalText: txt,
        readableText: txt.replace(/\\n/g, '\n').replace(/\\t/g, '\t'),
        expanded: true,
        editor: null
      }))
    },
    editorInit (e, obj) {
      obj.editor = e
    },
    copyYAML (obj, i) {
      const e = obj.editor
      e.setValue(obj.originalText)
      e.focus()
      e.selectAll()
      if (document.execCommand('copy')) {
        this.$message.success('复制成功')
      } else {
        this.$message.error('复制失败')
      }
      e.setValue(obj.readableText)
    },
    toggleYAML (obj) {
      obj.expanded = !obj.expanded
    },
    copyAllYAML () {
      const textArea = document.createElement('textarea')
      textArea.value = this.exportModal.textObjects
        .map(obj => obj.originalText)
        .join('\n\n---\n\n')
      document.body.appendChild(textArea)
      textArea.focus()
      textArea.select()

      if (document.execCommand('copy')) {
        this.$message.success('复制成功')
      } else {
        this.$message.error('复制失败')
      }

      document.body.removeChild(textArea)
    }
  },
  computed: {
    currentOrganizationId () {
      return this.$store.state.login.userinfo.organization.id
    },
    versionId () {
      return this.$route.params.id
    },
    showArtifactDeployBtn () {
      if (this.currentVersionDetail.deployInfo.length !== 0 && this.currentVersionDetail.buildInfo.length === 0) {
        return true
      } else {
        return false
      }
    },
    productName () {
      return this.$route.params.project_name
    },
    versionTag () {
      return this.$route.query.version
    }
  },
  created () {
    bus.$emit(`set-topbar-title`, {
      title: '',
      breadcrumb: [{ title: '版本管理', url: `` },
        { title: this.productName, url: `/v1/delivery/version/${this.productName}` },
        { title: this.versionTag, url: `` }]
    })
    this.getVersionDetail()
  },
  components: {
    editor: aceEditor,
    runWorkflowFromVersion
  }
}
</script>

<style lang="less">
.issue-popper {
  display: inline-block;
  font-size: 14px;

  p {
    margin: 0.5em 0;
  }

  .issue-url {
    color: #1989fa;
    cursor: pointer;
  }
}

.version-container {
  position: relative;
  flex: 1;
  padding: 15px 30px;
  overflow: auto;
  font-size: 13px;

  .el-table td,
  .el-table th {
    padding: 5px 0;
  }

  .img-link {
    color: #1989fa;
  }

  .module-title h1 {
    margin-bottom: 1.5rem;
    font-weight: 200;
    font-size: 2rem;
  }

  .text {
    font-size: 13px;
  }

  .item {
    padding: 10px 0;
    padding-left: 1px;

    .icon-color {
      color: #9ea3a9;
      cursor: pointer;

      &:hover {
        color: #1989fa;
      }
    }

    .icon-color-cancel {
      color: #ff4949;
      cursor: pointer;
    }

    .status {
      line-height: 24px;
    }
  }

  .clearfix::before,
  .clearfix::after {
    display: table;
    content: "";
  }

  .clearfix {
    span {
      color: #999;
      font-size: 16px;
      line-height: 20px;
    }
  }

  .clearfix::after {
    clear: both;
  }

  .el-tabs {
    .el-tabs__header {
      .el-tabs__nav {
        width: 100%;

        .el-tabs__item {
          &.is-disabled {
            float: right;

            span {
              color: #1989fa;
              cursor: pointer;
            }
          }
        }
      }
    }
  }

  .box-card {
    background-color: #fff;

    .item-title {
      color: #8d9199;
    }

    .item-desc {
      .start-build,
      .edit-pipeline {
        color: #1989fa;
        font-size: 13px;
        cursor: pointer;
      }
    }

    .color-passed {
      color: #6ac73c;
      font-weight: 500;
    }

    .color-failed {
      color: #ff1949;
      font-weight: 500;
    }

    .color-cancelled {
      color: #909399;
      font-weight: 500;
    }

    .color-timeout {
      color: #e6a23c;
      font-weight: 500;
    }

    .color-running {
      color: #1989fa;
      font-weight: 500;
    }

    .color-created {
      color: #cdb62c;
      font-weight: 500;
    }

    .color-notrunning {
      color: #303133;
      font-weight: 500;
    }

    .issue-tag {
      margin-right: 5px;
      margin-bottom: 5px;
      cursor: pointer;
    }

    .deploy_env {
      color: #1989fa;
    }

    .error-color {
      color: #fa6464;
    }

    .show-log {
      font-size: 1.33rem;
      cursor: pointer;

      &:hover {
        color: #1989fa;
      }
    }

    .show-test-result {
      color: #1989fa;
      cursor: pointer;
    }

    .show-log-title {
      line-height: 17px;
    }

    .operation {
      line-height: 16px;
    }
  }

  .box-card,
  .task-process {
    border: none;
    box-shadow: none;
  }

  .task-process {
    width: 100%;
  }

  .el-card__header {
    padding-top: 0;
    padding-bottom: 5px;
    padding-left: 0;
  }

  .el-card__body {
    padding: 2px 20px;
  }

  .el-row {
    margin-bottom: 15px;

    &:last-child {
      margin-bottom: 0;
    }
  }

  .link,
  .issue-name-wrapper {
    display: block;

    a {
      color: #1989fa;
      cursor: pointer;
    }
  }

  .section-head {
    width: 222px;
    height: 28px;
    margin-bottom: 15px;
    color: #666;
    font-size: 14px;
    line-height: 28px;
    border-bottom: 1px solid #eee;
  }

  a.item-desc {
    color: #1989fa;
    cursor: pointer;
  }

  .test-report-link {
    a {
      color: #1989fa;
    }
  }

  .export-dialog {
    .op-row {
      display: flex;
      margin-bottom: 5px;

      .el-icon-caret-bottom {
        transform: rotate(-90deg);
        transition: transform 0.3s ease-out;
      }

      .at-right {
        display: none;
        margin-left: auto;
      }

      &.expanded {
        .el-icon-caret-bottom {
          transform: rotate(0);
        }

        .at-right {
          display: inline;
        }
      }

      .el-button--small {
        height: 32px;
      }
    }

    .nothing {
      color: #000;
    }
  }

  .pointer {
    cursor: pointer;
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
}
</style>
