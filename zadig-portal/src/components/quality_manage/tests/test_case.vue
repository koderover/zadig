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
        { title: '测试管理', url: '/v1/tests' },
        { title: this.projectName, url: `/v1/tests/detail/${this.projectName}/test` },
        { title: '功能测试', url: `/v1/tests/detail/${this.projectName}/test/function` },
        { title: this.workflowName, url: `/v1/tests/detail/${this.projectName}/test/detail/function/${this.workflowName}` },
        { title: `#${this.taskId}`, url: '' }]
    })
  },
  components: {
    TestCase
  }
}
</script>
