<template>
  <div class="projects-detail-container">
    <div class="projects-detail-sub"
         v-loading="detailLoading"
         element-loading-text="加载中..."
         element-loading-spinner="iconfont iconfont-loading iconxiangmu">
      <div class="project-header">
        <div class="header-start">
          <div class="container">
            <div class="display-mode">
              <div class="btn-container">
                <el-dropdown placement="bottom"
                             @command="selectSystemToDownloadCLI">
                  <button type="button"
                          class="display-btn">
                    下载开发者 CLI
                    <i class="el-icon-arrow-down el-icon--right"></i>
                  </button>
                  <el-dropdown-menu slot="dropdown">
                    <el-dropdown-item disabled>选择使用的系统 </el-dropdown-item>
                    <el-dropdown-item command="mac"> Mac </el-dropdown-item>
                    <el-dropdown-item command="linux"> Linux </el-dropdown-item>
                    <el-dropdown-item command="windows"> Windows </el-dropdown-item>
                  </el-dropdown-menu>
                </el-dropdown>
                <router-link :to="`/v1/projects/edit/${projectName}`">
                  <button type="button"
                          class="display-btn">
                    <i class="el-icon-edit-outline"></i>
                    <span class="add-filter-value-title">修改</span>
                  </button>
                </router-link>
                <button type="button"
                        @click="deleteProject"
                        class="display-btn">
                  <i class="el-icon-delete"></i>
                  <span class="add-filter-value-title">删除</span>
                </button>
              </div>

            </div>
          </div>
        </div>
      </div>
      <div class="projects-detail">
        <section class="basic">
          <div class="info">
            <h4 class="section-title"
                style="margin-top: 0;">
              基本信息
              <el-popover trigger="hover"
                          placement="right">
                <div class="project-desc-show">
                  <h4>管理员</h4>
                  <p>
                    {{projectAdminArray.length ? projectAdminArray.join(' , ') : 'N/A'}}
                  </p>
                  <h4>描述</h4>
                  <p class="desc-show">{{currentProject && currentProject.desc}}</p>
                </div>
                <i slot="reference"
                   class="el-icon-warning-outline"></i>
              </el-popover>
            </h4>
            <div class="info-list">
              <el-row type="flex"
                      justify="space-between">
                <el-col :span="4">
                  <router-link :to="`/v1/projects/detail/${projectName}/pipelines`">
                    <div class="card">
                      <div class="flex">
                        <i class="icon iconfont icongongzuoliucheng"></i>
                        <div class="text-base card-title ">工作流</div>
                      </div>
                      <div class="number font-bold  mt-6">
                        {{currentProject.total_workflow_num}}
                        <span>
                          条
                        </span>
                      </div>
                      <div class="flex">
                        <div class="card-footer">
                          <div class="btn-container">
                          </div>
                        </div>
                      </div>
                    </div>
                  </router-link>
                </el-col>
                <el-col :span="4">
                  <router-link :to="`/v1/projects/detail/${projectName}/envs`">
                    <div class="card">

                      <div class="flex">
                        <i class="icon iconfont iconrongqi"></i>
                        <div class="text-base card-title ">环境</div>
                      </div>
                      <div class="number font-bold  mt-6">{{currentProject.total_env_num}}
                        <span>
                          个
                        </span>
                      </div>
                      <div class="flex">
                        <div class="card-footer">
                          <div class="btn-container">

                          </div>
                        </div>
                      </div>
                    </div>
                  </router-link>
                </el-col>
                <el-col :span="4">
                  <router-link :to="`/v1/projects/detail/${projectName}/services`">
                    <div class="card">
                      <div class="flex">
                        <i class="icon iconfont iconrongqifuwu"></i>
                        <div class="text-base card-title ">服务</div>
                      </div>
                      <div class="number font-bold  mt-6">
                        {{currentProject.total_service_num}}
                        <span>
                          个
                        </span>
                      </div>

                      <div class="flex">
                        <div class="card-footer">
                        </div>
                      </div>
                    </div>
                  </router-link>
                </el-col>
                <el-col :span="4">
                  <router-link :to="`/v1/projects/detail/${projectName}/builds`">
                    <div class="card">

                      <div class="flex">
                        <i class="icon iconfont icongoujianzhong"></i>
                        <div class="text-base card-title ">构建</div>
                      </div>
                      <div class="number font-bold  mt-6">{{currentProject.total_build_num}}
                        <span>
                          个
                        </span>
                      </div>
                      <div class="flex">
                        <div class="card-footer">
                          <div class="btn-container">

                          </div>
                        </div>
                      </div>
                    </div>
                  </router-link>
                </el-col>
                <el-col :span="4">
                  <router-link :to="`/v1/projects/detail/${projectName}/test`">
                    <div class="card">

                      <div class="flex">
                        <i class="icon iconfont icontest"></i>
                        <div class="text-base card-title ">测试</div>
                      </div>
                      <div class="number font-bold  mt-6">{{currentProject.total_test_num}}
                        <span>
                          个
                        </span>
                      </div>
                      <div class="flex">
                        <div class="card-footer">
                          <div class="btn-container">
                          </div>
                        </div>
                      </div>
                    </div>
                  </router-link>
                </el-col>
              </el-row>
            </div>
          </div>
        </section>
        <section class="status">
          <div class="env">
            <h4 class="section-title">环境信息</h4>
            <div class="env-list">
              <el-table :data="envList"
                        stripe
                        style="width: 100%;">
                <el-table-column label="环境名称">

                  <template slot-scope="scope">
                    <router-link
                                 :to="`/v1/projects/detail/${scope.row.product_name}/envs/detail?envName=${scope.row.env_name}`">
                      <span class="env-name">{{`${scope.row.env_name}`}}</span>
                    </router-link>
                  </template>
                </el-table-column>
                <el-table-column label="集群归属">
                  <template slot-scope="scope">
                    <span v-if="scope.row.clusterName">{{`${scope.row.clusterName}`}}</span>
                    <span v-else>{{`${scope.row.clusterType}`}}</span>
                  </template>
                </el-table-column>
                <el-table-column label="当前状态">
                  <template slot-scope="scope">
                    <span
                          v-if="scope.row.status">{{getProdStatus(scope.row.status,scope.row.updatable)}}</span>
                    <span v-else><i class="el-icon-loading"></i></span>
                  </template>
                </el-table-column>
                <el-table-column width="300"
                                 label="更新信息（时间/操作人）">
                  <template slot-scope="scope">
                    <div><i class="el-icon-time"></i>
                      {{ $utils.convertTimestamp(scope.row.update_time) }} <i
                         class="el-icon-user"></i>
                      <span>{{scope.row.update_by}}</span>
                    </div>
                  </template>
                </el-table-column>
              </el-table>
            </div>
          </div>
          <div class="workflow">
            <h4 class="section-title">工作流信息</h4>
            <div class="workflow-info-list">
              <el-table :data="workflows"
                        stripe
                        style="width: 100%;">
                <el-table-column label="工作流名称">
                  <template slot-scope="scope">
                    <router-link class="pipeline-name"
                                 :to="`/v1/projects/detail/${projectName}/pipelines/multi/${scope.row.name}`">
                      {{scope.row.name}}
                    </router-link>
                  </template>
                </el-table-column>
                <el-table-column label="包含步骤">
                  <section slot-scope="scope">
                    <span>
                      <span
                            v-if="!$utils.isEmpty(scope.row.build_stage) && scope.row.build_stage.enabled">
                        <el-tag size="small">构建部署</el-tag>
                        <span v-if="scope.row.test_stage.enabled||scope.row.distribute_stage.enabled"
                              class="step-arrow"><i class="el-icon-right"></i></span>
                      </span>
                      <span
                            v-if="!$utils.isEmpty(scope.row.artifact_stage) && scope.row.artifact_stage.enabled">
                        <el-tag size="small">交付物部署</el-tag>
                        <span v-if="scope.row.test_stage.enabled||scope.row.distribute_stage.enabled"
                              class="step-arrow"><i class="el-icon-right"></i></span>
                      </span>
                      <span
                            v-if="!$utils.isEmpty(scope.row.test_stage) && scope.row.test_stage.enabled">
                        <el-tag size="small">测试</el-tag>
                        <span v-if="scope.row.distribute_stage && scope.row.distribute_stage.enabled"
                              class="step-arrow"><i class="el-icon-right"></i></span>
                      </span>
                      <el-tag v-if="!$utils.isEmpty(scope.row.distribute_stage) &&  scope.row.distribute_stage.enabled"
                              size="small">分发</el-tag>
                    </span>
                  </section>
                </el-table-column>
                <el-table-column label="当前状态">
                  <template slot-scope="scope">
                    <span>{{ wordTranslation(scope.row.lastest_task.status,'pipeline','task')}}</span>
                  </template>
                </el-table-column>
                <el-table-column width="300"
                                 label="更新信息（时间/操作人）">
                  <template slot-scope="scope">
                    <div><i class="el-icon-time"></i>
                      {{ $utils.convertTimestamp(scope.row.update_time) }} <i
                         class="el-icon-user"></i>
                      {{scope.row.update_by}}
                    </div>
                  </template>
                </el-table-column>
              </el-table>
            </div>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>
<script>
import { getProductInfo, getBuildConfigsAPI, deleteProjectAPI, getClusterListAPI, getProjectInfoAPI, listProductAPI, usersAPI, downloadDevelopCLIAPI } from '@api'
import { mapGetters } from 'vuex'
import { getProductStatus } from '@utils/word_translate'
import { wordTranslate } from '@utils/word_translate.js'
import { whetherOnboarding } from '@utils/onboarding_route'
import bus from '@utils/event_bus'
import _ from 'lodash'
export default {
  data () {
    return {
      currentProject: {},
      envList: [],
      allCluster: [],
      buildConfigs: [],
      detailLoading: true,
      usersList: []
    }
  },
  methods: {
    getBuildConfig () {
      const projectName = this.projectName
      getBuildConfigsAPI(projectName).then((res) => {
        this.buildConfigs = res
      })
    },
    getProdStatus (status, updateble) {
      return getProductStatus(status, updateble)
    },
    getEnvList () {
      const projectName = this.projectName
      listProductAPI('', projectName).then((res) => {
        this.envList = res.map(element => {
          getProductInfo(projectName, element.env_name).then((res) => {
            element.status = res.status
          })
          if (element.cluster_id) {
            element.clusterType = this.getClusterType(element.cluster_id).type
            element.clusterName = this.getClusterType(element.cluster_id).name
          } else {
            element.clusterName = ''
            element.clusterType = '本地'
          }
          return element
        })
      })
    },
    getCluster () {
      getClusterListAPI().then((res) => {
        this.allCluster = res
      })
    },
    getClusterType (clusterId) {
      if (clusterId && this.allCluster.length > 0) {
        const clusterObj = this.allCluster.find(cluster => cluster.id === clusterId)
        if (clusterObj && clusterObj.production) {
          return {
            type: '生产',
            name: clusterObj.name
          }
        } else if (clusterObj && clusterObj.production === false) {
          return {
            type: '测试',
            name: clusterObj.name
          }
        }
      } else {
        return {
          type: '本地',
          name: ''
        }
      }
    },
    deleteProject () {
      const services = _.flattenDeep(this.currentProject.services)
      const envNames = this.envList.map((element) => { return element.env_name })
      const buildConfigs = this.buildConfigs.map((element) => { return element.name })
      const workflows = this.workflows.map((element) => { return element.name })
      const allWorkflows = workflows
      const htmlTemplate = `
      <span><b>服务：</b>${services.length > 0 ? services.join(', ') : '无'}</span><br>
      <span><b>构建：</b>${buildConfigs.length > 0 ? buildConfigs.join(', ') : '无'}</span><br>
      <span><b>环境：</b>${envNames.length > 0 ? envNames.join(', ') : '无'}</span><br>
      <span><b>工作流：</b>${allWorkflows.length > 0 ? allWorkflows.join(', ') : '无'}</span>
      `
      const projectName = this.projectName
      this.$prompt(`该项目下的资源会同时被删除<span style="color:red">请谨慎操作！！</span><br> ${htmlTemplate}`, `请输入项目名 ${projectName} 确认删除`, {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        dangerouslyUseHTMLString: true,
        customClass: 'product-prompt',
        confirmButtonClass: 'el-button el-button--danger',
        inputValidator: project_name => {
          if (project_name === projectName) {
            return true
          } else if (project_name === '') {
            return '请输入项目名'
          } else {
            return '项目名不相符'
          }
        }
      })
        .then(({ value }) => {
          deleteProjectAPI(projectName).then(
            response => {
              this.$message({
                type: 'success',
                message: '项目删除成功'
              })
              this.$router.push('/v1/projects')
            }
          )
        })
        .catch(() => {
          this.$message({
            type: 'info',
            message: '取消删除'
          })
        })
    },
    wordTranslation (word, category, subitem) {
      return wordTranslate(word, category, subitem)
    },
    getProject (projectName) {
      getProjectInfoAPI(projectName).then((res) => {
        this.currentProject = res
        if (res.onboarding_status) {
          this.$router.push(whetherOnboarding(res))
        }
        bus.$emit('set-sub-sidebar-title', {
          title: this.projectName,
          url: `/v1/projects/detail/${this.projectName}`,
          routerList: [
            { name: '工作流', url: `/v1/projects/detail/${this.projectName}/pipelines` },
            { name: '集成环境', url: `/v1/projects/detail/${this.projectName}/envs` },
            { name: '服务', url: `/v1/projects/detail/${this.projectName}/services` },
            { name: '构建', url: `/v1/projects/detail/${this.projectName}/builds` },
            { name: '测试', url: `/v1/projects/detail/${this.projectName}/test` }
          ]
        })
        this.detailLoading = false
      })
    },
    getUserList () {
      const orgId = this.currentOrganizationId
      usersAPI(orgId).then((res) => {
        this.usersList = res.data
      })
    },
    selectSystemToDownloadCLI (check) {
      downloadDevelopCLIAPI(check).then(res => {
        const aEle = document.createElement('a')
        if (aEle.download !== undefined) {
          aEle.setAttribute('href', res)
          aEle.setAttribute('download', true)
          document.body.appendChild(aEle)
          aEle.click()
          document.body.removeChild(aEle)
        }
      })
    }
  },
  computed: {
    projectName () {
      return this.$route.params.project_name
    },
    workflows () {
      const list = this.$utils.filterObjectArrayByKey('name', '', this.workflowList)
      return list.filter(w => w.product_tmpl_name === this.projectName)
    },
    ...mapGetters([
      'workflowList'
    ]),
    currentOrganizationId () {
      return this.$store.state.login.userinfo.organization.id
    },
    projectAdminArray () {
      return this.usersList
        ? this.usersList.filter(userInfo => {
          return this.currentProject.user_ids ? this.currentProject.user_ids.includes(userInfo.id) : false
        }).map(user => {
          return user.name
        })
        : []
    },
    isProjectAdmin () {
      if (this.$utils.roleCheck().superAdmin) {
        return true
      }
      return this.currentProject.user_ids ? this.currentProject.user_ids.includes(this.$store.state.login.userinfo.info.id) : false
    }
  },
  components: {
  },
  created () {
    this.getProject(this.projectName)
  },
  mounted () {
    this.$store.dispatch('refreshWorkflowList')
    this.getEnvList()
    this.getCluster()
    this.getBuildConfig()
    bus.$emit('show-sidebar', true)
    bus.$emit('set-topbar-title', { title: '', breadcrumb: [{ title: '项目', url: '/v1/projects' }, { title: this.projectName, url: '' }] })
    bus.$emit('set-sub-sidebar-title', {
      title: '',
      routerList: []
    })
    this.getUserList()
  }
}
</script>

<style lang="less" >
.projects-detail-container {
  position: relative;
  flex: 1;
  overflow: auto;
  background-color: #f5f7f7;

  .projects-detail-sub {
    min-height: 100%;
  }

  .project-header {
    display: flex;
    align-items: stretch;
    justify-content: flex-start;

    .header-start {
      flex: 1;

      .container {
        min-height: 40px;
        margin: 0;
        padding: 5px 20px 0 20px;
        font-size: 13px;

        .display-mode {
          display: flex;
          flex-wrap: wrap;
          align-items: baseline;
          justify-content: flex-end;
          min-height: 46px;

          .btn-container {
            position: relative;
            height: 44px;
            margin-top: 1px;
            margin-right: 5px;

            .display-btn {
              padding: 13px 17px;
              color: #1989fa;
              font-size: 13px;
              text-decoration: none;
              background-color: #fff;
              border: none;
              border-color: #fff;
              border-style: none;
              border-radius: 2px;
              box-shadow: 0 4px 4px rgba(0, 0, 0, 0.05);
              cursor: pointer;

              &:hover {
                color: #1989fa;
                background-color: #fff;
                border-color: #1989fa;
              }

              &.active {
                color: #fff;
                background-color: #1989fa;
                border-color: #1989fa;
              }
            }
          }
        }
      }
    }

    .header-end {
      .add-project-btn {
        width: 165px;
        height: 100%;
        padding: 10px 17px;
        color: #fff;
        font-size: 13px;
        text-decoration: none;
        background-color: #1989fa;
        border: 1px solid #1989fa;
        cursor: pointer;
      }
    }
  }

  .projects-detail {
    padding: 0 20px 50px 20px;

    .section-title {
      margin: 10px;
      color: #4c4c4c;
    }

    .info-list {
      .el-col-4 {
        width: 19%;
      }

      .card {
        position: relative;
        padding: 0.75rem 0.55rem;
        background-color: #fff;
        background-color: rgba(255, 255, 255, 1);
        border-radius: 0.375rem;
        box-shadow: 0 3px 20px #0000000b;
        cursor: pointer;
        transition: transform 0.4s;

        &:hover {
          box-shadow:
            0 20px 25px -5px rgba(0, 0, 0, 0.1),
            0 10px 10px -5px rgba(0, 0, 0, 0.04);
          transform: scale(1.03);
        }

        .flex {
          display: flex;

          .icon {
            color: #3160d8;
            font-size: 25px;
          }

          .card-footer {
            margin: auto;

            .btn-container {
              display: flex;
              align-items: center;
              color: #1989fa;
              font-weight: 500;
              font-size: 0.75rem;
              border-radius: 9999px;

              a {
                color: #1989fa;
              }

              a + a {
                margin-left: 8px;
              }

              .el-button {
                color: #1989fa;
                font-weight: 400;
                border: 1px solid #1989fa;

                &:hover {
                  color: #fff;
                  background: #1989fa;

                  a {
                    color: #fff;
                  }
                }

                &.el-button--mini,
                &.el-button--mini.is-round {
                  padding: 6px 12px;
                }
              }
            }
          }

          .text-theme-10 {
            color: rgba(49, 96, 216, 1);
          }

          .report-box__icon {
            width: 28px;
            height: 28px;
          }
        }

        .mt-6 {
          margin: 1.3rem 0;
        }

        .card-title {
          margin-top: 0.25rem;
          margin-left: 0.25rem;
          color: #718096;
        }

        .number {
          color: #000;
          font-size: 2.1rem;
          line-height: 2rem;
          text-align: center;

          span {
            color: #718096;
            font-size: 13px;
          }
        }

        .font-bold {
          font-weight: 500;
        }
      }
    }

    .el-table {
      .step-arrow {
        color: #409eff;
      }

      .project-name {
        color: #4c4c4c;
        font-weight: bold;
        font-size: 16px;
        text-align: left;
      }

      .pipeline-name,
      .env-name,
      .resource-name {
        color: #1989fa;
      }

      .operation {
        margin-right: 10px;
        padding: 0 5px;
        color: #606266;

        &:hover {
          color: #1989fa;
        }
      }
    }
  }
}

.el-message-box.product-prompt {
  width: 40%;

  .el-message-box__content {
    max-height: 300px;
    overflow-y: auto;
  }
}

.el-popover {
  overflow: hidden;

  .project-desc-show {
    width: 230px;
    max-height: 230px;
    margin: -18px 0 -10px;
    overflow: auto;
    border-bottom: 10px solid white;

    h4 {
      margin-bottom: -8px;
      line-height: 1.2;
    }

    p {
      padding: 0 10px;

      &.desc-show {
        white-space: pre-wrap;
        word-break: break-all;
      }
    }
  }
}
</style>
