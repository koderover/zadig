<template>
  <div class="mobile-pipelines-list">
    <van-nav-bar fixed>
      <template #title>
        工作流列表
      </template>
    </van-nav-bar>
    <form action="/">
      <van-search v-model="keyword"
                  input-align="center"
                  placeholder="请输入搜索关键词"
                  @search="onSearch" />
    </form>
    <van-cell-group>
      <van-cell v-for="(workflow,index) in filteredWorkflows"
                center
                border
                :key="index"
                :to="`/mobile/pipelines/project/${workflow.product_tmpl_name}/multi/${workflow.name}`">
        <template #title>
          <span class="workflow-name">{{`${workflow.name}`}}</span>
        </template>
        <template #label>
          <van-tag v-if="!$utils.isEmpty(workflow.build_stage) && workflow.build_stage.enabled"
                   type="primary">构建部署</van-tag>
          <van-tag v-if="!$utils.isEmpty(workflow.artifact_stage) && workflow.artifact_stage.enabled"
                   type="primary">交付物部署</van-tag>
          <van-tag v-if="!$utils.isEmpty(workflow.distribute_stage) &&  workflow.distribute_stage.enabled"
                   type="primary">分发</van-tag>
        </template>
        <template #default>
          <van-icon @click.stop.prevent="showAction(workflow)"
                    name="more-o"
                    size="2em"
                    color="#1989fa" />
        </template>

      </van-cell>
    </van-cell-group>
    <van-action-sheet v-model="currentAction.show"
                      cancel-text="取消"
                      close-on-click-action
                      :actions="actions"
                      @select="onSelectAction" />
    <el-dialog :visible.sync="taskDialogVisible"
               title="运行 产品-工作流"
               custom-class="run-workflow"
               width="100%"
               class="dialog">
      <run-workflow v-if="taskDialogVisible"
                    :workflowName="workflowToRun.name"
                    :workflowMeta="workflowToRun"
                    :targetProduct="workflowToRun.product_tmpl_name"
                    @success="hideTaskDialog"></run-workflow>
    </el-dialog>
  </div>
</template>
<script>
import { NavBar, Tag, Panel, Loading, Button, Notify, Tab, Tabs, Cell, CellGroup, Icon, Empty, Search, Toast, ActionSheet } from 'vant';
import qs from 'qs';
import _ from 'lodash';
import runWorkflow from './run_workflow.vue';
export default {
  components: {
    [NavBar.name]: NavBar,
    [Tag.name]: Tag,
    [Panel.name]: Panel,
    [Loading.name]: Loading,
    [Button.name]: Button,
    [Notify.name]: Notify,
    [Tab.name]: Tab,
    [Tabs.name]: Tabs,
    [Cell.name]: Cell,
    [CellGroup.name]: CellGroup,
    [Icon.name]: Icon,
    [Empty.name]: Empty,
    [Search.name]: Search,
    [Toast.name]: Toast,
    [ActionSheet.name]: ActionSheet,
    runWorkflow
  },
  data() {
    return {
      keyword: '',
      loading: false,
      workflowToRun: {},
      taskDialogVisible: false,
      actions: [
        { name: '启动' },
      ],
      currentAction: {
        show: false,
        workflow_name: ''
      }
    }
  },
  computed: {
    workflows() {
      return this.$store.getters.workflowList;
    },
    filteredWorkflows() {
      this.$router.replace({
        query: Object.assign(
          {},
          qs.parse(window.location.search, { ignoreQueryPrefix: true }),
          {
            name: this.keyword
          })
      });
      let list = this.$utils.filterObjectArrayByKey('name', this.keyword, this.workflows);
      return list;
    },
  },
  methods: {
    onSearch(val) {
      Toast(val);
    },
    onSelectAction(action) {
      if (action.name === '启动') {
        this.taskDialogVisible = true;
      }
      else if (action.name === '删除') {

      }
    },
    fetchWorkflows() {
      this.loading = true;
      this.$store.dispatch('refreshWorkflowList').then(() => {
        this.loading = false;
      });
    },
    showAction(workflow) {
      this.workflowToRun = workflow;
      this.$set(this.currentAction, 'show', true);
      this.$set(this.currentAction, 'workflow_name', workflow.name);
    },
    hideTaskDialog() {
      this.taskDialogVisible = false;
    },
  },
  mounted() {
    this.fetchWorkflows();
  },
}
</script>
<style lang="less">
.mobile-pipelines-list {
  padding-top: 46px;
  padding-bottom: 50px;
}
</style>