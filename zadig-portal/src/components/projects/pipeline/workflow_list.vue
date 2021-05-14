<template>
  <div class="workflow-list"
       ref="workflow-list">
    <div>
      <ul class="workflow-ul">
        <div class="project-header">
          <div class="header-start">
            <div class="container">
              <div class="function-container">
                <div class="btn-container">
                  <button type="button"
                          :class="{'active':showFavorite}"
                          @click="showFavorite=!showFavorite"
                          class="display-btn">
                    <i class="el-icon-star-off favorite"></i>
                  </button>
                  <el-input v-model="keyword"
                            placeholder="搜索工作流"
                            class="search-workflow"
                            prefix-icon="el-icon-search"
                            clearable></el-input>
                </div>
              </div>
            </div>
          </div>
          <div class="header-end">
            <router-link
                         :to="`/productpipelines/create?projectName=${this.projectName ? this.projectName : ''}`">
              <button type="button"
                      class="add-project-btn">
                <i class="el-icon-plus"></i>
                新建工作流
              </button>
            </router-link>
          </div>
        </div>
        <div v-loading="loading"
             class="pipeline-loading"
             element-loading-text="加载中..."
             element-loading-spinner="iconfont iconfont-loading icongongzuoliucheng">
          <virtual-list v-if="availableWorkflows.length > 0"
                        class="virtual-list-container"
                        :data-key="'name'"
                        :data-sources="availableWorkflows"
                        :data-component="itemComponent"
                        :keeps="20"
                        :estimate-size="72">
          </virtual-list>
          <div v-if="availableWorkflows.length === 0"
               class="no-product">
            <img src="@assets/icons/illustration/pipeline.svg"
                 alt="">
            <p>暂无可展示的工作流，请手动添加工作流</p>
          </div>
        </div>
      </ul>

    </div>

    <el-dialog title="运行 产品-工作流"
               :visible.sync="showStartProductBuild"
               custom-class="run-workflow"
               width="60%">
      <run-workflow v-if="showStartProductBuild"
                    :workflowName="workflowToRun.name"
                    :workflowMeta="workflowToRun"
                    :targetProduct="workflowToRun.product_tmpl_name"
                    @success="hideProductTaskDialog"></run-workflow>
    </el-dialog>

  </div>
</template>

<script>
import virtualListItem from './workflow_list/virtual_list_item';
import runWorkflow from './common/run_workflow.vue';
import virtualList from 'vue-virtual-scroll-list';
import qs from 'qs';
import { deleteWorkflowAPI, copyWorkflowAPI } from '@api';
import bus from '@utils/event_bus';
import _ from 'lodash';
import { mapGetters } from 'vuex';

export default {
  data() {
    return {
      itemComponent: virtualListItem,
      showStartProductBuild: false,
      loading: false,
      showFavorite: false,
      workflowToRun: {},
      remain: 10,
      keyword: '',
      selectedType: '',
    };
  },
  provide() {
    return {
      startProductBuild: this.startProductBuild,
      copyWorkflow: this.copyWorkflow,
      deleteWorkflow: this.deleteWorkflow,
      renamePipeline: this.renamePipeline,
    }
  },
  computed: {
    ...mapGetters([
      'getOnboardingTemplates'
    ]),
    projectName() {
      return this.$route.params.project_name;
    },
    workflows() {
      return this.$store.getters.workflowList;
    },
    availableWorkflows() {
      const availableWorkflows = this.filteredWorkflows;
      if (this.showFavorite) {
        const favoriteWorkflows = this.$utils.cloneObj(availableWorkflows).filter((x) => {
          return x.is_favorite;
        });
        return favoriteWorkflows;
      }
      else {
        const sortedByFavorite = this.$utils.cloneObj(availableWorkflows).sort((x) => {
          return x.is_favorite ? -1 : 1;
        });
        return sortedByFavorite;
      }
    },
    filteredWorkflows() {
      let list = this.$utils.filterObjectArrayByKey('name', this.keyword, this.workflows).filter(pipeline => {
        return !this.getOnboardingTemplates.includes(pipeline.product_tmpl_name);
      })
      if (this.projectName) {
        list = (list.filter(w => w.product_tmpl_name === this.projectName));
      }
      return list;
    },
  },
  watch: {
    keyword(val) {
      this.$router.replace({
        query: Object.assign(
          {},
          qs.parse(window.location.search, { ignoreQueryPrefix: true }),
          {
            name: val
          })
      });
    },
    projectName(val) {
      if (val) {
        bus.$emit(`set-topbar-title`, { title: '', breadcrumb: [{ title: '项目', url: '/v1/projects' }, { title: this.projectName, url: `/v1/projects/detail/${this.projectName}` }, { title: '工作流', url: '' }] });
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
      } else {
        bus.$emit(`show-sidebar`, true);
        bus.$emit(`set-topbar-title`, { title: '工作流', breadcrumb: [] });
        bus.$emit(`set-sub-sidebar-title`, {
          title: '',
          routerList: []
        });
      }
    },
  },
  methods: {
    fetchWorkflows() {
      this.loading = true;
      this.$store.dispatch('refreshWorkflowList').then(() => {
        this.loading = false;
      });
    },
    deleteWorkflow(name) {
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
          this.$store.dispatch('refreshWorkflowList');
        });
      });
    },
    startProductBuild(workflow) {
      this.workflowToRun = workflow;
      this.showStartProductBuild = true;
    },
    hideProductTaskDialog() {
      this.showStartProductBuild = false;
    },
    copyWorkflow(pipeline_name) {
      this.$prompt('请输入新的产品工作流名称', '复制工作流', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        inputValidator: new_name => {
          let pipeNames = [];
          this.workflows.forEach(element => {
            pipeNames.push(element.name);
          });
          if (new_name == '') {
            return '请输入工作流名称';
          } else if (pipeNames.includes(new_name)) {
            return '工作流名称重复';
          } else if (!/^[a-zA-Z0-9-]+$/.test(new_name)) {
            return '名称只支持字母大小写和数字，特殊字符只支持中划线';
          } else {
            return true;
          }
        }
      }).then(({ value }) => {
        this.copyWorkflowReq(pipeline_name, value);
      }).catch(() => {
        this.$message({
          type: 'info',
          message: '取消复制'
        });
      });
    },
    copyWorkflowReq(oldName, newName) {
      copyWorkflowAPI(oldName, newName).then(() => {
        this.$message({
          message: '复制流水线成功',
          type: 'success'
        });
        this.$store.dispatch('refreshWorkflowList');
        this.$router.push(`/productpipelines/edit/${newName}`);
      });
    },
  },
  created() {
    this.keyword = this.$route.query.name ? this.$route.query.name : '';
    this.fetchWorkflows();
    if (this.projectName) {
      bus.$emit(`set-topbar-title`, { title: '', breadcrumb: [{ title: '项目', url: '/v1/projects' }, { title: this.projectName, url: `/v1/projects/detail/${this.projectName}` }, { title: '工作流', url: '' }] });
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
    }
    else {
      bus.$emit(`show-sidebar`, true);
      bus.$emit(`set-topbar-title`, { title: '工作流', breadcrumb: [] });
      bus.$emit(`set-sub-sidebar-title`, {
        title: '',
        routerList: []
      });
    }
  },
  components: {
    runWorkflow,
    'virtual-list': virtualList
  },
};
</script>

<style lang="less">
.workflow-list {
  overflow-y: hidden;
  flex: 1;
  position: relative;
  padding-left: 1.07143rem;
  background-color: #f5f7f7;
  ::-webkit-scrollbar-track {
    border-radius: 6px;
    background-color: #f5f5f5;
  }
  ::-webkit-scrollbar {
    width: 5px;
    background-color: #f5f5f5;
  }
  ::-webkit-scrollbar-thumb {
    border-radius: 6px;
    background-color: #b7b8b9;
  }
  .pipeline-type-dialog {
    .choice,
    .desc {
      line-height: 32px;
    }
    .desc {
      padding-left: 24px;
      color: #999;
    }
  }
  .search-pipeline {
    width: 100%;
    display: inline-block;
    padding-top: 15px;
    .el-input {
      width: 200px;
      .el-input__inner {
        border-radius: 16px;
      }
    }
    .el-radio {
      margin-left: 15px;
    }
  }
  .project-header {
    display: flex;
    align-items: stretch;
    justify-content: flex-start;
    background-color: #f5f7f7;
    .header-start {
      flex: 1;
      .container {
        font-size: 13px;
        margin: 0;
        padding: 10px 20px;
        .function-container {
          display: flex;
          justify-content: flex-end;
          flex-wrap: wrap;
          min-height: 50px;
          .btn-container {
            margin-right: 5px;
            position: relative;
            display: flex;
            align-items: center;
            .search-workflow {
              width: auto;
            }
            .display-btn {
              font-size: 13px;
              background-color: #fff;
              cursor: pointer;
              text-decoration: none;
              border-color: #fff;
              border-radius: 4px;
              color: #1989fa;
              box-shadow: 0px 4px 4px rgba(0, 0, 0, 0.05);
              padding: 10px 10px;
              margin-right: 5px;
              border-style: none;
              .favorite {
                font-size: 20px;
              }
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
  .pipeline-loading {
    height: calc(~"100vh - 100px");
    .virtual-list-container {
      height: calc(~"100vh - 100px");
      overflow-y: auto;
    }
    .no-product {
      height: 70vh;
      display: flex;
      align-content: center;
      flex-direction: column;
      justify-content: center;
      align-items: center;
      img {
        width: 400px;
        height: 400px;
      }
      p {
        font-size: 15px;
        color: #606266;
      }
    }
    .button-exec {
      font-size: 16px;
      margin-right: 10px;
      padding: 8px 12px 8px 10px;
    }
  }
  .workflow-ul {
    list-style: none;
    margin: 0;
    background: #fff;
    padding: 0;
    .start-build {
      color: #000;
    }
    .more-operation {
      color: #000;
      font-size: 20px;
      cursor: pointer;
    }
    .step-arrow {
      color: #409eff;
    }
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
