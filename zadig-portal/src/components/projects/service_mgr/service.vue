<template>
  <component :is="currentComponent"
             :projectInfo="projectInfo"></component>
</template>
<script>
import { getSingleProjectAPI } from '@/api'
import bus from '@utils/event_bus'

export default {
  name: 'service',
  data () {
    return {
      projectInfo: {},
      projectName: null,
      currentComponent: null
    }
  },
  methods: {
    async checkProjectFeature () {
      const projectName = this.projectName
      this.projectInfo = await getSingleProjectAPI(projectName)
      if (this.projectInfo.product_feature) {
        if (this.projectInfo.product_feature.basic_facility === 'kubernetes') {
          if (this.projectInfo.product_feature.deploy_type === 'helm') {
            this.currentComponent = () => import('./service_helm')
          } else {
            this.currentComponent = () => import('./service_k8s')
          }
        } else if (this.projectInfo.product_feature.basic_facility === 'cloud_host') {
          this.currentComponent = () => import('./service_pm')
        }
      } else {
        this.currentComponent = () => import('./service_k8s')
      }
    }
  },
  created () {
    this.projectName = this.$route.params.project_name
    this.checkProjectFeature()

    bus.$emit(`show-sidebar`, false)
    bus.$emit(`set-topbar-title`, { title: '', breadcrumb: [{ title: '项目', url: '/v1/projects' }, { title: this.projectName, url: `/v1/projects/detail/${this.projectName}` }, { title: '服务管理', url: '' }] })
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
