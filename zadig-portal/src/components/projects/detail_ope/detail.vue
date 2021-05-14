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
                style="margin-top:0">
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
                      :gutter="20"
                      justify="space-between">
                <el-col :span="6">
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
                <el-col :span="6">
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
                <el-col :span="6">
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
                <el-col :span="6">
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
                        style="width: 100%">
                <el-table-column label="环境名称">

                  <template slot-scope="scope">
                    <router-link
                                 :to="`/v1/projects/detail/${scope.row.product_name}/envs/detail?envName=${scope.row.env_name}`">
                      <span class="env-name">{{`${scope.row.env_name}`}}</span>
                    </router-link>
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
                        style="width: 100%">
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
                        <span v-if="scope.row.distribute_stage.enabled"
                              class="step-arrow"><i class="el-icon-right"></i></span>
                      </span>
                      <span
                            v-if="!$utils.isEmpty(scope.row.artifact_stage) && scope.row.artifact_stage.enabled">
                        <el-tag size="small">交付物部署</el-tag>
                        <span v-if="scope.row.distribute_stage.enabled"
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
import { getProductInfo, getBuildConfigsAPI, deleteProjectAPI, getProjectInfoAPI, listProductAPI, usersAPI, downloadDevelopCLIAPI } from '@api';
import { mapGetters } from 'vuex';
import { getProductStatus } from '@utils/word_translate';
import { wordTranslate } from '@utils/word_translate.js';
import { whetherOnboarding } from '@utils/onboarding_route'
import bus from '@utils/event_bus';
import _ from 'lodash';
export default {
  data() {
    return {
      currentProject: {},
      envList: [],
      buildConfigs: [],
      detailLoading: true,
      usersList: []
    }
  },
  methods: {
    getBuildConfig() {
      const projectName = this.projectName;
      getBuildConfigsAPI(projectName).then((res) => {
        this.buildConfigs = res;
      });
    },
    getProdStatus(status, updateble) {
      return getProductStatus(status, updateble);
    },
    getEnvList() {
      const projectName = this.projectName;
      listProductAPI('', projectName).then((res) => {
        this.envList = res.map(element => {
          getProductInfo(projectName, element.env_name).then((res) => {
            element.status = res.status;
          })
          return element;
        });
      })
    },
    deleteProject() {
      const services = _.flattenDeep(this.currentProject.services);
      const envNames = this.envList.map((element) => { return element.env_name });
      const buildConfigs = this.buildConfigs.map((element) => { return element.name });
      const workflows = this.workflows.map((element) => { return element.name });
      const allWorkflows = workflows
      const htmlTemplate = `
      <span><b>服务：</b>${services.length > 0 ? services.join(', ') : '无'}</span><br>
      <span><b>构建：</b>${buildConfigs.length > 0 ? buildConfigs.join(', ') : '无'}</span><br>
      <span><b>环境：</b>${envNames.length > 0 ? envNames.join(', ') : '无'}</span><br>
      <span><b>工作流：</b>${allWorkflows.length > 0 ? allWorkflows.join(', ') : '无'}</span>
      `;
      const projectName = this.projectName;
      this.$prompt(`该项目下的资源会同时被删除<span style="color:red">请谨慎操作！！</span><br> ${htmlTemplate}`, `请输入项目名 ${projectName} 确认删除`, {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        dangerouslyUseHTMLString: true,
        customClass: 'product-prompt',
        confirmButtonClass: 'el-button el-button--danger',
        inputValidator: project_name => {
          if (project_name == projectName) {
            return true;
          } else if (project_name == '') {
            return '请输入项目名';
          } else {
            return '项目名不相符';
          }
        }
      })
        .then(({ value }) => {
          deleteProjectAPI(projectName).then(
            response => {
              this.$message({
                type: 'success',
                message: '项目删除成功'
              });
              this.$router.push('/v1/projects');
            }
          );
        })
        .catch(() => {
          this.$message({
            type: 'info',
            message: '取消删除'
          });
        });
    },
    wordTranslation(word, category, subitem) {
      return wordTranslate(word, category, subitem);
    },
    getProject(projectName) {
      getProjectInfoAPI(projectName).then((res) => {
        this.currentProject = res;
        if (res.onboarding_status) {
          this.$router.push(whetherOnboarding(res));
        }
        bus.$emit(`set-sub-sidebar-title`, {
          title: this.projectName,
          url: `/v1/projects/detail/${this.projectName}`,
          routerList: [
            { name: '工作流', url: `/v1/projects/detail/${this.projectName}/pipelines` },
            { name: '集成环境', url: `/v1/projects/detail/${this.projectName}/envs` },
            { name: '服务', url: `/v1/projects/detail/${this.projectName}/services` },
            { name: '构建', url: `/v1/projects/detail/${this.projectName}/builds` },
          ]
        });
        this.detailLoading = false;
      });
    },
    getUserList() {
      const orgId = this.currentOrganizationId;
      usersAPI(orgId).then((res) => {
        this.usersList = res.data;
      });
    },
    selectSystemToDownloadCLI(check) {
      downloadDevelopCLIAPI(check).then(res => {
        let aEle = document.createElement('a');
        if (aEle.download !== undefined) {
          aEle.setAttribute('href', res);
          aEle.setAttribute('download', true);
          document.body.appendChild(aEle);
          aEle.click();
          document.body.removeChild(aEle);
        }
      })
    }
  },
  computed: {
    projectName() {
      return this.$route.params.project_name;
    },
    workflows() {
      let list = this.$utils.filterObjectArrayByKey('name', '', this.workflowList);
      return list.filter(w => w.product_tmpl_name === this.projectName);
    },
    ...mapGetters([
      'workflowList'
    ]),
    currentOrganizationId() {
      return this.$store.state.login.userinfo.organization.id;
    },
    projectAdminArray() {
      return this.usersList
        ? this.usersList.filter(userInfo => {
          return this.currentProject.user_ids ? this.currentProject.user_ids.includes(userInfo.id) : false;
        }).map(user => {
          return user.name
        })
        : [];
    },
    isProjectAdmin() {
      if (this.$utils.roleCheck().superAdmin) {
        return true;
      }
      return this.currentProject.user_ids ? this.currentProject.user_ids.includes(this.$store.state.login.userinfo.info.id) : false;
    },
  },
  components: {
  },
  created() {
    this.getProject(this.projectName);
  },
  mounted() {
    this.$store.dispatch('refreshWorkflowList').then(() => { });
    this.getEnvList();
    this.getBuildConfig();
    bus.$emit(`show-sidebar`, true);
    bus.$emit(`set-topbar-title`, { title: '', breadcrumb: [{ title: '项目', url: '/v1/projects' }, { title: this.projectName, url: '' }] });
    bus.$emit(`set-sub-sidebar-title`, {
      title: '',
      routerList: []
    });
    this.getUserList();
  }
};
</script>

<style lang="less" >
.projects-detail-container {
  flex: 1;
  position: relative;
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
        font-size: 13px;
        margin: 0;
        min-height: 40px;
        padding: 5px 20px 0px 20px;
        .display-mode {
          display: flex;
          align-items: baseline;
          justify-content: flex-end;
          flex-wrap: wrap;
          min-height: 46px;
          .btn-container {
            height: 44px;
            margin-right: 5px;
            margin-top: 1px;
            position: relative;
            .display-btn {
              font-size: 13px;
              background-color: #fff;
              cursor: pointer;
              text-decoration: none;
              border-color: #fff;
              color: #1989fa;
              box-shadow: 0px 4px 4px rgba(0, 0, 0, 0.05);
              padding: 13px 17px;
              border: none;
              border-radius: 2px;
              border-style: none;
              &:hover {
                border-color: #1989fa;
                background-color: #fff;
                color: #1989fa;
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
        text-decoration: none;
        background-color: #1989fa;
        color: #fff;
        padding: 10px 17px;
        border: 1px solid #1989fa;
        font-size: 13px;
        cursor: pointer;
        width: 165px;
        height: 100%;
      }
    }
  }
  .projects-detail {
    padding: 0 20px 50px 20px;
    .section-title {
      color: #4c4c4c;
      margin: 10px;
    }
    .info-list {
      .el-col-4 {
        width: 19%;
      }
      .card {
        padding: 0.75rem 0.55rem;
        box-shadow: 0px 3px 20px #0000000b;
        background-color: #fff;
        background-color: rgba(255, 255, 255, 1);
        border-radius: 0.375rem;
        transition: transform 0.4s;
        position: relative;
        cursor: pointer;
        &:hover {
          transform: scale(1.03);
          box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1),
            0 10px 10px -5px rgba(0, 0, 0, 0.04);
        }
        .flex {
          display: flex;
          .icon {
            font-size: 25px;
            color: #3160d8;
          }
          .card-footer {
            margin: auto;
            .btn-container {
              display: flex;
              border-radius: 9999px;
              color: #1989fa;
              a {
                color: #1989fa;
              }
              a + a {
                margin-left: 8px;
              }
              font-size: 0.75rem;
              align-items: center;
              font-weight: 500;
              .el-button {
                border: 1px solid #1989fa;
                color: #1989fa;
                font-weight: 400;
                &:hover {
                  color: #fff;
                  a {
                    color: #fff;
                  }
                  background: #1989fa;
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
          color: #718096;
          margin-top: 0.25rem;
          margin-left: 0.25rem;
        }
        .number {
          font-size: 2.1rem;
          text-align: center;
          line-height: 2rem;
          color: #000;
          span {
            font-size: 13px;
            color: #718096;
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
        font-size: 16px;
        font-weight: bold;
        text-align: left;
        color: #4c4c4c;
      }
      .pipeline-name,
      .env-name,
      .resource-name {
        color: #1989fa;
      }
      .operation {
        margin-right: 10px;
        color: #606266;
        padding: 0px 5px;
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
    margin: -18px 0px -10px;
    width: 230px;
    max-height: 230px;
    overflow: auto;
    border-bottom: 10px solid white;
    h4 {
      line-height: 1.2;
      margin-bottom: -8px;
    }
    p {
      padding: 0 10px;
      &.desc-show {
        word-break: break-all;
        white-space: pre-wrap;
      }
    }
  }
}
</style>
