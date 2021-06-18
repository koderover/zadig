<template>
  <function-task-detail :basePath="`projects`"></function-task-detail>
</template>

<script>
import FunctionTaskDetail from '@/components/projects/test/common/function/function_task_detail.vue'
import bus from '@utils/event_bus'
export default {
  data () {
    return {
    }
  },
  computed: {
    workflowName () {
      return this.$route.params.test_name
    },
    taskID () {
      return this.$route.params.task_id
    },
    projectName () {
      return this.$route.params.project_name
    }
  },
  methods: {
    setTitleBar () {
      bus.$emit(`set-topbar-title`, {
        title: '',
        breadcrumb: [
          { title: '项目', url: '/v1/projects' },
          { title: this.projectName, url: `/v1/projects/detail/${this.projectName}` },
          { title: '功能测试', url: `/v1/projects/detail/${this.projectName}/test/function` },
          { title: this.workflowName, url: `/v1/projects/detail/${this.projectName}/test/detail/function/${this.workflowName}` },
          { title: `#${this.taskID}`, url: '' }]
      })
      bus.$emit(`set-sub-sidebar-title`, {
        title: this.projectName,
        url: `/v1/projects/detail/${this.projectName}`,
        routerList: [
          { name: '工作流', url: `/v1/projects/detail/${this.projectName}/pipelines` },
          { name: '集成环境', url: `/v1/projects/detail/${this.projectName}/envs` },
          { name: '服务', url: `/v1/projects/detail/${this.projectName}/services` },
          { name: '构建', url: `/v1/projects/detail/${this.projectName}/builds` },
          { name: '测试', url: `/v1/projects/detail/${this.projectName}/test` }]
      })
    }
  },
  watch: {
    $route (to, from) {
      this.setTitleBar()
    }
  },
  created () {
    this.setTitleBar()
  },
  components: {
    FunctionTaskDetail
  }
}
</script>
