<template>
  <div class="workflow-detail">
    <el-card class="box-card wide"
             :body-style="{ padding: '0px', margin: '15px 0 0 0' }">
      <div slot="header"
           class="block-title">
        基本信息
      </div>
      <div class="text item">
        <el-row :gutter="0">
          <el-col :span="4">
            <div class="grid-content item-title"><i class="el-icon-user-solid"></i> 修改人</div>
          </el-col>
          <el-col :span="4">
            <div class="grid-content item-desc">{{ workflow.update_by }}</div>
          </el-col>
        </el-row>
        <el-row v-if="workflow.description"
                :gutter="0">
          <el-col :span="4">
            <div class="grid-content item-title"><i class="el-icon-chat-line-square"></i> 描述</div>
          </el-col>
          <el-col :span="8">
            <div class="grid-content item-desc">{{ workflow.description }}</div>
          </el-col>
        </el-row>
        <el-row :gutter="0">
          <el-col :span="4">
            <div class="grid-content item-title"><i class="el-icon-time"></i> 更新时间</div>
          </el-col>
          <el-col :span="6">
            <div class="grid-content item-desc">
              {{ $utils.convertTimestamp(workflow.update_time) }}
            </div>
          </el-col>
        </el-row>
        <el-row :gutter="0">
          <el-col :span="4">
            <div class="grid-content item-title process"><i class="el-icon-finished"></i> 流程</div>
          </el-col>
          <el-col :span="20">
            <div class="grid-content process">
              <ul>
                <span v-if="!$utils.isEmpty(workflow.build_stage) && workflow.build_stage.enabled">
                  <el-tag size="small">构建部署</el-tag>
                  <span v-if="workflow.distribute_stage.enabled"
                        class="step-arrow"><i class="el-icon-right"></i></span>
                </span>
                <span
                      v-if="!$utils.isEmpty(workflow.artifact_stage) && workflow.artifact_stage.enabled">
                  <el-tag size="small">交付物部署</el-tag>
                  <span v-if="workflow.distribute_stage.enabled"
                        class="step-arrow"><i class="el-icon-right"></i></span>
                </span>
                <el-tag v-if="!$utils.isEmpty(workflow.distribute_stage) &&  workflow.distribute_stage.enabled"
                        size="small">分发</el-tag>
              </ul>
            </div>
          </el-col>
        </el-row>
        <el-row :gutter="0">
          <el-col :span="4">
            <div class="grid-content item-title operation"><i class="el-icon-s-operation"></i> 操作
            </div>
          </el-col>
          <el-col :span="8">
            <div class="grid-content item-desc">
              <el-tooltip effect="dark"
                          content="执行"
                          placement="top">
                <i @click="startTask"
                   class="el-icon-video-play start-build"></i>
              </el-tooltip>
              <template>
                <el-tooltip effect="dark"
                            content="编辑工作流"
                            placement="top">
                  <router-link :to="`/productpipelines/edit/${workflowName}`"
                               class="not-anchor">
                    <i class="el-icon-edit-outline edit-pipeline"></i>
                  </router-link>
                </el-tooltip>
                <el-tooltip effect="dark"
                            content="删除工作流"
                            placement="top">
                  <i @click="removeWorkflow"
                     class="el-icon-delete edit-pipeline"></i>
                </el-tooltip>
              </template>

            </div>
          </el-col>
        </el-row>
      </div>
    </el-card>

    <el-card class="box-card full"
             :body-style="{ padding: '0px', margin: '15px 0 30px 0' }">
      <div slot="header"
           class="block-title">
        历史任务
      </div>
      <task-list :taskList="workflowTasks"
                 :total="total"
                 :pageSize="pageSize"
                 :projectName="projectName"
                 :baseUrl="`/v1/projects/detail/${projectName}/pipelines/multi/${workflowName}`"
                 :workflowName="workflowName"
                 @cloneTask="rerun"
                 @currentChange="changeTaskPage"
                 showEnv
                 showServiceNames
                 showOperation>
      </task-list>
    </el-card>

    <el-dialog :visible.sync="taskDialogVisible"
               title="运行 产品-工作流"
               custom-class="run-workflow"
               width="60%"
               class="dialog">
      <run-workflow v-if="taskDialogVisible"
                    :workflowName="workflowName"
                    :workflowMeta="workflow"
                    :targetProduct="workflow.product_tmpl_name"
                    :forcedUserInput="forcedUserInput"
                    @success="hideAndFetchHistory"></run-workflow>
    </el-dialog>
  </div>
</template>

<script>
import { workflowAPI, deleteWorkflowAPI, workflowTaskListAPI } from '@api';
import runWorkflow from './common/run_workflow.vue';
import bus from '@utils/event_bus';
export default {
  data() {
    return {
      workflow: {},
      workflowTasks: [],
      total: 0,
      pageSize: 50,
      taskDialogVisible: false,
      durationSet: {},
      forcedUserInput: {},
      pageStart: 0,
      timerId: null,
      timeTimeoutFinishFlag: false
    };
  },
  computed: {
    workflowName() {
      return this.$route.params.workflow_name;
    },
    projectName() {
      return this.$route.params.project_name;
    },
  },
  methods: {
    async refreshHistoryTask() {
      const res = await workflowTaskListAPI(this.workflowName, this.pageStart, this.pageSize);
      this.workflowTasks = res.data;
      this.total = res.total;
      if (!this.timeTimeoutFinishFlag) {
        this.timerId = setTimeout(this.refreshHistoryTask, 3000)
      }
    },
    fetchHistory(start, max) {
      workflowTaskListAPI(this.workflowName, start, max).then(res => {
        this.workflowTasks = res.data;
        this.total = res.total;
      });
    },
    changeTaskPage(val) {
      const start = (val - 1) * this.pageSize;
      this.pageStart = start;
      this.fetchHistory(start, this.pageSize);
    },
    hideAndFetchHistory() {
      this.taskDialogVisible = false;
      this.fetchHistory(0, this.pageSize);
    },

    startTask() {
      this.taskDialogVisible = true;
      this.forcedUserInput = {};
    },
    removeWorkflow() {
      const name = this.workflowName;
      this.$prompt('输入工作流名称确认', '删除工作流 ' + name, {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        confirmButtonClass: 'el-button el-button--danger',
        inputValidator: pipe_name => {
          if (pipe_name === name) {
            return true;
          } else if (pipe_name === '') {
            return '请输入工作流名称';
          } else {
            return '名称不相符';
          }
        }
      }).then(({ value }) => {
        deleteWorkflowAPI(name).then(() => {
          this.$message.success('删除成功');
          this.$router.push(`/v1/projects/detail/${this.projectName}/pipelines`);
          this.$store.dispatch('refreshWorkflowList');
        });
      });
    },
    rerun(task) {
      this.taskDialogVisible = true;

      const args = task.workflow_args;
      this.forcedUserInput = {
        workflow_name: args.workflow_name,
        product_tmpl_name: args.product_tmpl_name,
        description: args.description,
        namespace: args.namespace,
        targets: args.targets,
        tests: args.tests,
      };
      if (args.artifact_args && args.artifact_args.length > 0) {
        Object.assign(this.forcedUserInput, {
          artifactArgs: args.artifact_args,
          versionArgs: args.version_args,
          registryId: args.registry_id,
        })
      }
    },
  },
  beforeDestroy() {
    this.timeTimeoutFinishFlag = true;
    clearTimeout(this.timerId);
  },
  mounted() {
    workflowAPI(this.workflowName).then(res => {
      this.workflow = res;
    });
    this.refreshHistoryTask();
    bus.$emit(`set-topbar-title`, {
      title: '',
      breadcrumb: [
        { title: '项目', url: '/v1/projects' },
        { title: this.projectName, url: `/v1/projects/detail/${this.projectName}` },
        { title: '工作流', url: `/v1/projects/detail/${this.projectName}/pipelines` },
        { title: this.workflowName, url: '' }]
    });
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
  },
  components: {
    runWorkflow,
  },
};
</script>

<style lang="less">
.workflow-detail {
  background-color: #fff;
  flex: 1;
  position: relative;
  overflow: auto;
  padding: 0px 30px;
  font-size: 13px;
  .text {
    font-size: 13px;
  }
  .item {
    padding: 10px 0;
    padding-left: 1px;
    .icon-color {
      cursor: pointer;
      color: #9ea3a9;
      &:hover {
        color: #1989fa;
      }
    }
    .icon-color-cancel {
      cursor: pointer;
      color: #ff4949;
    }
  }
  .clearfix:before,
  .clearfix:after {
    display: table;
    content: "";
  }
  .block-title {
    line-height: 20px;
    color: #999;
    font-size: 16px;
  }
  .clearfix:after {
    clear: both;
  }
  .box-card {
    width: 600px;
    background-color: #fff;
    .item-title {
      color: #8d9199;
    }
    .process {
      line-height: 24px;
    }
    .operation {
      line-height: 18px;
    }
    .item-desc {
      .start-build,
      .edit-pipeline {
        cursor: pointer;
        font-size: 1.3rem;
        margin-right: 0.3em;
        &:hover {
          color: #1989fa;
        }
      }
      .favorite {
        display: inline-block;
        cursor: pointer;
        color: #69696bb3;
        &.liked {
          color: #f4e118;
        }
        &:hover {
          color: #f4e118;
        }
      }
    }
    .task-id,
    .report-link {
      color: #1989fa;
    }
    .process {
      ul {
        line-height: 1;
        margin: 0;
        padding: 0;
        li {
          list-style: none;
          line-height: 24px;
          display: inline-block;
          cursor: pointer;
        }
        .step-arrow {
          color: #409eff;
        }
      }
      .dot {
        width: 12px;
        height: 12px;
        background: #d1d9e5;
        border-radius: 50%;
        vertical-align: middle;
      }
      .active {
        background-color: #1989fa;
      }
      .build {
        background-color: #fa4c7e;
      }
      .deploy {
        background-color: #fdd243;
      }
      .test {
        background-color: #78da59;
      }
      .distribution {
        background-color: #166cd6;
      }
    }
    .pagination {
      display: flex;
      align-items: center;
      justify-content: center;
      margin-top: 20px;
    }
  }
  .box-card,
  .box-card-stack {
    margin-top: 15px;
    box-shadow: none;
    border: none;
  }
  .wide {
    width: 65%;
  }
  .full {
    width: 100%;
  }
  .el-card__header {
    padding-left: 0px;
  }
  .el-row {
    margin-bottom: 15px;
    &:last-child {
      margin-bottom: 0;
    }
  }

  .pipeline-edit {
    .el-dialog__body {
      padding: 15px 20px;
      .el-form {
        .el-form-item {
          margin-bottom: 15px;
        }
      }
    }
  }
  .fileTree-dialog {
    .el-dialog__body {
      padding: 0px 5px;
    }
  }

  .buildv2-edit-form {
    .el-form-item__label {
      padding: 0px;
      font-size: 13px;
      line-height: 25px;
    }
  }
  .not-anchor {
    color: unset;
  }
  .run-workflow {
    .el-dialog__body {
      padding: 8px 10px;
      color: #606266;
      font-size: 14px;
    }
  }
}
</style>
