<template>
  <function-task-detail :basePath="`tests`"></function-task-detail>
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
    projectName () {
      return this.$route.params.project_name
    },
    taskID () {
      return this.$route.params.task_id
    }
  },
  methods: {
    setTitleBar () {
      bus.$emit(`set-topbar-title`, {
        title: '',
        breadcrumb: [
          { title: '测试管理', url: '/v1/tests' },
          { title: this.projectName, url: `/v1/tests/detail/${this.projectName}/test` },
          { title: '功能测试', url: `/v1/tests/detail/${this.projectName}/test/function` },
          { title: this.workflowName, url: `/v1/tests/detail/${this.projectName}/test/detail/function/${this.workflowName}` },
          { title: `#${this.taskID}`, url: '' }]
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
