<template>
  <div>
    <pipeline-row :name="workflow.name"
                  :isFavorite="workflow.is_favorite"
                  :type="'workflow'"
                  :productName="workflow.product_tmpl_name"
                  :pipelineLink="`/v1/projects/detail/${workflow.product_tmpl_name}/pipelines/multi/${workflow.name}`"
                  :latestTaskStatus="workflow.lastest_task.status"
                  :recentSuccessID="workflow.last_task_success.task_id"
                  :avgRuntime="makeProductAvgRunTime(workflow.total_duration,workflow.total_num)"
                  :avgSuccessRate="makeProductAvgSuccessRate(workflow.total_success,workflow.total_num)"
                  :recentSuccessLink="makeProductTaskDetailLink(workflow.product_tmpl_name,workflow.last_task_success)"
                  :recentFailID="workflow.last_task_failure.task_id"
                  :recentFailLink="makeProductTaskDetailLink(workflow.product_tmpl_name,workflow.last_task_failure)"
                  :updateBy="workflow.update_by"
                  :updateTime="$utils.convertTimestamp(workflow.update_time)">
      <section slot="more"
               class="dash-process">
        <span>
          <span v-if="!$utils.isEmpty(workflow.build_stage) && workflow.build_stage.enabled">
            <el-tag size="mini">构建部署</el-tag>
          </span>
          <span v-if="!$utils.isEmpty(workflow.artifact_stage) && workflow.artifact_stage.enabled">
            <el-tag size="mini">交付物部署</el-tag>
          </span>
          <el-tag v-if="!$utils.isEmpty(workflow.distribute_stage) &&  workflow.distribute_stage.enabled"
                  size="mini">分发</el-tag>
        </span>
      </section>
      <template slot="operations">
        <el-button type="success"
                   class="button-exec"
                   @click="startProductBuild(workflow)">
          <span class="el-icon-video-play">&nbsp;执行</span>
        </el-button>
        <router-link :to="`/productpipelines/edit/${workflow.name}`">
          <span class="menu-item el-icon-setting start-build"></span>
        </router-link>
        <el-dropdown @visible-change="(status) => fnShowTimer(status, index, workflow)">
          <span class="el-dropdown-link">
            <i class="el-icon-s-operation more-operation"></i>
          </span>
          <el-dropdown-menu slot="dropdown">
            <el-dropdown-item @click.native="changeSchedule">
              <span>{{workflow.schedule_enabled ? '关闭': '打开'}}定时器</span>
            </el-dropdown-item>
            <el-dropdown-item @click.native="copyWorkflow(workflow.name)">
              <span>复制</span>
            </el-dropdown-item>
            <el-dropdown-item @click.native="deleteWorkflow(workflow.name)">
              <span>删除</span>
            </el-dropdown-item>
          </el-dropdown-menu>
        </el-dropdown>
      </template>
    </pipeline-row>
  </div>
</template>

<script>
import pipelineRow from './pipeline_row.vue';
import { workflowAPI, updateWorkflowAPI } from '@api';
export default {
  name: 'item-component',
  data() {
    return {
      pipelineInfo: null,
    }
  },
  props: {
    index: {
      type: Number
    },
    source: {
      type: Object,
      default() {
        return {}
      }
    }
  },
  inject: [
    'startProductBuild',
    'copyWorkflow',
    'deleteWorkflow',
    'renamePipeline'],
  computed: {
    workflow() {
      return this.source
    }
  },
  methods: {
    makeProductAvgRunTime(duration, total) {
      if (total > 0) {
        return (duration / total).toFixed(1) + 's';
      }
      else {
        return '-'
      }
    },
    makeProductAvgSuccessRate(success, total) {
      if (total > 0) {
        return ((success / total) * 100).toFixed(2) + "%";
      }
      else {
        return '-'
      }
    },
    makeProductTaskDetailLink(product_tmpl_name, taskInfo) {
      return `/v1/projects/detail/${product_tmpl_name}/pipelines/multi/${taskInfo.pipeline_name}/${taskInfo.task_id}?status=${taskInfo.status}`;
    },
    async fnShowTimer(status, index, workflow) {
      if (status && !workflow.showTimer) {
        this.pipelineInfo = await workflowAPI(workflow.name).catch(error => console.log(error))
        if (_.get(this.pipelineInfo, 'schedules.items', '[]').length) {
          this.$set(this.source, 'showTimer', true)
          this.$forceUpdate()
        }
      }
    },
    async changeSchedule() {
      let pipelineInfo = this.pipelineInfo
      pipelineInfo.schedule_enabled = !pipelineInfo.schedule_enabled
      let res = await updateWorkflowAPI(this.pipelineInfo).catch(error => console.log(error))
      if (res) {
        if (pipelineInfo.schedule_enabled) {
          this.$message.success('定时器开启成功');
        } else {
          this.$message.success('定时器关闭成功');
        }
        this.$store.dispatch('refreshWorkflowList');
      }
    },
  },
  components: {
    pipelineRow
  }
}
</script>
