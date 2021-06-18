<template>
  <test-case></test-case>
</template>

<script>
import TestCase from '@/components/projects/test/common/report/test_case.vue'
import bus from '@utils/event_bus'
export default {
  data () {
    return {
    }
  },
  components: {
    TestCase
  },
  computed: {
    workflowName () {
      return this.$route.params.workflow_name
    },
    projectName () {
      return this.$route.params.project_name
    },
    taskId () {
      return this.$route.params.task_id
    }
  },
  created () {
    bus.$emit(`set-topbar-title`, {
      title: '',
      breadcrumb: [
        { title: '项目', url: '/v1/projects' },
        { title: this.projectName, url: `/v1/projects/detail/${this.projectName}` },
        { title: '功能测试', url: `/v1/projects/detail/${this.projectName}/test/function` },
        { title: this.workflowName, url: `/v1/projects/detail/${this.projectName}/test/detail/function/${this.workflowName}` },
        { title: `#${this.taskId}`, url: `/v1/projects/detail/${this.projectName}/test/detail/function/${this.workflowName}/${this.taskId}` },
        { title: '测试用例', url: '' }]
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
}
</script>
