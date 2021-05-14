<template>
  <div class="projects-delivery-container">
    <div class="guide-container">
      <step :activeStep="3">
      </step>
      <div class="current-step-container">
        <div class="title-container">
          <span class="first">第四步</span>
          <span class="second">运行工作流触发服务的自动化交付</span>
        </div>
        <div class="account-integrations cf-block__list">
          <el-table v-loading="loading"
                    :data="mapWorkflows"
                    style="width: 100%">
            <el-table-column label="工作流名称">
              <template slot-scope="scope">
                <span style="margin-left: 10px">{{ scope.row.name }}</span>
              </template>
            </el-table-column>
            <el-table-column width="200px"
                             label="环境信息">
              <template slot-scope="scope">
                <a v-if="scope.row.env_name"
                   class="env-name"
                   :href="`/v1/projects/detail/${ scope.row.product_tmpl_name}/envs/detail?envName=${ scope.row.env_name}`"
                   target="_blank">{{ `${scope.row.product_tmpl_name}-env-${scope.row.env_name}` }}</a>
              </template>
            </el-table-column>
            <el-table-column label="服务入口">
              <template slot-scope="scope">
                <div v-for="(ingress,ingress_index) in scope.row.ingress_infos"
                     :key="ingress_index">
                  <div v-for="(item,host_index) in scope.row.ingress_infos[ingress_index]['host_info']"
                       :key="host_index">
                    <a style="color:#1989fa"
                       :href="`http://${item.host}`"
                       target="_blank">{{item.host}}</a>
                  </div>
                </div>
              </template>
            </el-table-column>
            <el-table-column width="200px"
                             label="包含步骤">
              <template slot-scope="scope">
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
              </template>
            </el-table-column>
            <el-table-column width="150px"
                             label="更新信息（时间/操作人）">
              <template slot-scope="scope">
                {{$utils.convertTimestamp(scope.row.update_time)}}
              </template>
            </el-table-column>
            <el-table-column width="120px"
                             label="操作">
              <template slot-scope="scope">
                <el-button type="success"
                           size="mini"
                           round
                           @click="runCurrentTask(scope.row)"
                           plain>点击运行</el-button>
              </template>
            </el-table-column>
          </el-table>

        </div>
      </div>
      <div class="other-operation">
        <h3 class="pipelines-aside-help__step-header">
          你可能还需要：
        </h3>
        <ul class="pipelines-aside-help__step-list">
          <li class="pipelines-aside-help__step-list-item"><a target="_blank"
               href="https://docs.koderover.com/zadig/project/workflow/#git-webhook"
               class="pipelines-aside-help__step-list-item-link"><i class="icon el-icon-link"></i>
              <span class="pipelines-aside-help__step-list-item-link-text">
                配置 Git Webhook 自动触发服务升级</span></a></li>
        </ul>
      </div>
    </div>
    <div class="controls__wrap">
      <div class="controls__right">
        <router-link :to="`/v1/projects/detail/${projectName}`">
          <button type="primary"
                  size="small"
                  class="save-btn"
                  :disabled="loading"
                  plain>完成</button>
        </router-link>
      </div>
    </div>
    <el-dialog :visible.sync="taskDialogVisible"
               title="运行 产品-工作流"
               custom-class="run-workflow"
               width="60%"
               class="dialog">
      <run-workflow v-if="taskDialogVisible"
                    :workflowName="workflow.name"
                    :workflowMeta="workflow"
                    :targetProduct="workflow.product_tmpl_name"
                    @success="hideAfterSuccess"></run-workflow>
    </el-dialog>
  </div>
</template>
<script>
import bus from '@utils/event_bus';
import step from '../common/step.vue';
import runWorkflow from '../../pipeline/common/run_workflow.vue';
import { getProjectIngressAPI } from '@api';
import _ from 'lodash';
export default {
  data() {
    return {
      loading: true,
      workflow: {},
      taskDialogVisible: false,
      mapWorkflows: []
    }
  },
  methods: {
    getWorkflows() {
      this.loading = true;
      this.$store.dispatch('refreshWorkflowList').then(() => {
        this.loading = false;
      }).then(() => {
        const projectName = this.projectName;
        const w1 = 'workflow-qa';
        const w2 = 'workflow-dev';
        const w3 = 'workflow-ops';
        let currentWorkflows = this.$store.getters.workflowList.filter(element => {
          return (element.name.includes(w1) && element.product_tmpl_name === this.projectName) || (element.name.includes(w2) && element.product_tmpl_name === this.projectName) || (element.name.includes(w3) && element.product_tmpl_name === this.projectName);
        }).map((ele) => {
          let element = Object.assign({}, ele);
          element.name.includes(w1) ? element.env_name = `qa` : '';
          element.name.includes(w2) ? element.env_name = `dev` : '';
          element.name.includes(w3) ? element.env_name = '' : '';
          return element;
        })
        getProjectIngressAPI(projectName).then((res) => {
          currentWorkflows.map(workflow => {
            res.forEach(ingress => {
              if (ingress.env_name === workflow.env_name) {
                workflow.ingress_infos = ingress.ingress_infos;
              }
            })

          });
          this.mapWorkflows = currentWorkflows;
        })
      });
    },
    runCurrentTask(scope) {
      this.workflow = scope;
      this.taskDialogVisible = true;
    },
    hideAfterSuccess() {
      this.taskDialogVisible = false;
    }
  },
  computed: {
    projectName() {
      return this.$route.params.project_name;
    },
  },
  created() {
    this.getWorkflows();
    bus.$emit(`set-topbar-title`, { title: '', breadcrumb: [{ title: '项目', url: '/v1/projects' }, { title: this.projectName, url: '' }] });
    bus.$emit(`set-sub-sidebar-title`, {
      title: '',
      routerList: []
    });
  },
  components: {
    step, runWorkflow,
  },
  onboardingStatus: 0
}
</script>

<style lang="less">
.projects-delivery-container {
  flex: 1;
  position: relative;
  overflow: auto;
  background-color: #f5f7f7;

  .page-title-container {
    display: flex;
    padding: 0 20px;
    h1 {
      text-align: center;
      width: 100%;
      color: #4c4c4c;
      font-weight: 300;
    }
  }
  .guide-container {
    margin-top: 10px;
    min-height: calc(~"100% - 70px");
    .current-step-container {
      .title-container {
        margin-left: 20px;
        .first {
          font-size: 18px;
          background: #3289e4;
          color: #fff;
          font-weight: 300;
          padding: 8px;
          display: inline-block;
          width: 110px;
          text-align: center;
        }
        .second {
          font-size: 13px;
          color: #4c4c4c;
        }
      }

      .cf-block__list {
        -webkit-box-flex: 1;
        -ms-flex: 1;
        flex: 1;
        overflow-y: auto;
        background-color: inherit;
        padding: 0 30px;
        margin-top: 15px;
        .title {
          h4 {
            color: #4c4c4c;
            margin: 10px 0;
            font-weight: 400;
            text-decoration: underline;
          }
        }
        .env-name {
          color: #1989fa;
        }
        .cf-block__item {
          min-height: 102px;
          .account-box-item {
            display: -webkit-box;
            display: -ms-flexbox;
            display: flex;
            -webkit-box-align: center;
            -ms-flex-align: center;
            align-items: center;
            -webkit-box-pack: justify;
            -ms-flex-pack: justify;
            justify-content: space-between;
            margin-bottom: 10px;
            padding: 20px 30px;
            background-color: #fff;
            -webkit-box-shadow: 0 3px 2px 1px rgba(0, 0, 0, 0.05);
            box-shadow: 0 3px 2px 1px rgba(0, 0, 0, 0.05);
            filter: progid:DXImageTransform.Microsoft.dropshadow(OffX=0, OffY=3px, Color='#0D000000');
            .integration-card {
              display: -webkit-box;
              display: -ms-flexbox;
              display: flex;
              -webkit-box-align: center;
              -ms-flex-align: center;
              align-items: center;
              -webkit-box-pack: start;
              -ms-flex-pack: start;
              justify-content: flex-start;
              .integration-card__image {
                width: 64px;
                .el-button.is-circle {
                  border-radius: 50%;
                  padding: 6px;
                }
              }
              .cf-sub-title {
                font-size: 16px;
                font-weight: bold;
                text-align: left;
                color: #2f2f2f;
              }
              .integration-details {
                color: #4c4c4c;
                line-height: 20px;
                font-size: 14px;
              }
            }
            .integration-card > * {
              -webkit-box-flex: 0;
              -ms-flex: 0 0 auto;
              flex: 0 0 auto;
            }
          }
        }
      }
    }
    .other-operation {
      margin: 50px 30px 0 30px;
      .pipelines-aside-help__step-header {
        text-transform: uppercase;
        color: #000;
        font-size: 14px;
        font-weight: bold;
        margin: 0;
        width: 100%;
        margin: 10px 0;
      }
      .pipelines-aside-help__step-list {
        list-style: none;
        width: 100%;
        display: -webkit-box;
        display: -ms-flexbox;
        display: flex;
        -webkit-box-pack: start;
        -ms-flex-pack: start;
        justify-content: flex-start;
        -webkit-box-align: center;
        -ms-flex-align: center;
        align-items: center;
        -webkit-box-orient: vertical;
        -webkit-box-direction: normal;
        -ms-flex-direction: column;
        flex-direction: column;
        padding: 0;
        margin: 10px 0;
        -ms-flex-negative: 0;
        flex-shrink: 0;
        .pipelines-aside-help__step-header {
          text-transform: uppercase;
          color: #000;
          font-size: 12px;
          font-weight: bold;
          margin: 0;
          width: 100%;
          margin: 10px 0;
        }
        .pipelines-aside-help__step-list-item {
          display: -webkit-box;
          display: -ms-flexbox;
          display: flex;
          width: 100%;
          -webkit-box-align: start;
          -ms-flex-align: start;
          align-items: flex-start;
          -webkit-box-pack: start;
          -ms-flex-pack: start;
          justify-content: flex-start;
          margin-bottom: 8px;
          ul {
            padding-left: 15px;
          }
          ul > li {
            color: #606266;
          }
          .pipelines-aside-help__step-list-item-counter {
            height: 20px;
            width: 20px;
            border-radius: 50%;
            background-color: #9b51e0;
            color: #fff;
            font-size: 13px;
            font-weight: bold;
            display: -webkit-box;
            display: -ms-flexbox;
            display: flex;
            -webkit-box-align: center;
            -ms-flex-align: center;
            align-items: center;
            -webkit-box-pack: center;
            -ms-flex-pack: center;
            justify-content: center;
            margin-right: 13px;
          }
          .pipelines-aside-help__step-list-item-text {
            -webkit-box-flex: 1;
            -ms-flex: 1;
            flex: 1;
            color: #000;
            font-size: 13px;
            line-height: 24px;
            margin: 0;
          }
          .pipelines-aside-help__step-list-item-link,
          .pipelines-aside-help__step-list-item-link:hover,
          .pipelines-aside-help__step-list-item-link:focus,
          .pipelines-aside-help__step-list-item-link:active {
            color: #518ff6;
            text-decoration: none;
            .icon {
              margin-right: 5px;
            }
            .pipelines-aside-help__step-list-item-link-text {
              font-size: 12px;
            }
          }
        }
      }
    }
  }
  .alert {
    display: flex;
    padding: 0 25px;
    .el-alert {
      margin-bottom: 35px;
      .el-alert__title {
        font-size: 15px;
      }
    }
  }
  .controls__wrap {
    position: relative;
    bottom: 0;
    left: 0;
    right: 0;
    height: 60px;
    background-color: #fff;
    padding: 0 10px;
    z-index: 2;
    margin: 0 15px;
    box-shadow: 0 4px 4px 0 rgba(0, 0, 0, 0.05);
    display: flex;
    align-items: center;
    justify-content: space-between;
    > * {
      margin-right: 10px;
    }
    .controls__right {
      display: -webkit-box;
      display: -ms-flexbox;
      display: flex;
      -webkit-box-align: center;
      -ms-flex-align: center;
      align-items: center;
      .save-btn {
        text-decoration: none;
        background-color: #1989fa;
        color: #fff;
        padding: 10px 17px;
        border: 1px solid #1989fa;
        font-size: 13px;
        font-weight: bold;
        transition: background-color 300ms, color 300ms, border 300ms;
        cursor: pointer;
        margin-right: 15px;
      }
      .save-btn[disabled] {
        background-color: #9ac9f9;
        border: 1px solid #9ac9f9;
        cursor: not-allowed;
      }
    }
  }
}
</style>