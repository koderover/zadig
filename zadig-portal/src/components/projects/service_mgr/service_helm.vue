<template>
  <div class="content">
    <Code :service="serviceList"  >
    </Code>
  </div>
</template>
<script>
import Code from '../web_code/code'
export default {
  name: 'service_helm',
  components: {
    Code
  },
  data () {
    return {
      serviceList: []
    }
  },
  methods: {
    async querytHelmChartService () {
      this.$store.dispatch('queryService', { projectName: this.projectName })
    }
  },
  computed: {
    projectName () {
      return this.$route.params.project_name
    }
  },
  mounted () {
    this.querytHelmChartService()
  }
}
</script>
<style lang="less" scoped>
.content {
  display: flex;
  width: 100%;
  height: 100%;
  overflow: auto;
  background-color: #fff;
  user-select: none;
}
</style>
