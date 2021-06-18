<template>
  <div v-loading="loading"
       element-loading-text="加载中..."
       element-loading-spinner="iconfont iconfont-loading icontest"
       class="version-container-list">
    <div v-if="loading || noProjects"
         class="no-show">
      <img src="@assets/icons/illustration/test_manage.svg"
           alt="" />
      <p v-if="noProjects">暂无可展示的测试，请创建新的项目以添加测试</p>
    </div>
    <div v-else>
      <router-view></router-view>
    </div>
  </div>
</template>
<script>
import { mapGetters } from 'vuex'
import bus from '@utils/event_bus'
export default {
  data () {
    return {
      loading: true,
      noProjects: false
    }
  },
  computed: {
    ...mapGetters([
      'getTemplates'
    ]),
    projectName () {
      return this.$route.params.project_name
    }
  },
  methods: {
    async getProjects () {
      await this.$store.dispatch('refreshProjectTemplates')
      const routerList = this.getTemplates.map(element => {
        return { name: element.product_name, url: `/v1/tests/detail/${element.product_name}/test` }
      })
      bus.$emit(`set-topbar-title`, { title: '测试管理', breadcrumb: [] })
      bus.$emit(`set-sub-sidebar-title`, {
        title: '项目列表',
        routerList: routerList
      })
      this.loading = false
      if (routerList.length > 0) {
        if (!this.projectName) {
          this.$router.replace(`/v1/tests/detail/${routerList[0].name}/test/function`)
        }
      } else {
        this.noProjects = true
      }
    }
  },
  watch: {
    $route (to, from) {
      if (to.path === '/v1/tests') {
        this.$router.replace(`/v1/tests/detail/${this.getTemplates[0].product_name}/test/function`)
        return
      }
      if (to.path.split('/').length === 6) {
        this.$router.replace(`/v1/tests/detail/${this.projectName}/test/function`)
      }
    }
  },
  created () {
    this.getProjects()
  }
}
</script>

<style lang="less">
.version-container-list {
  position: relative;
  flex: 1;
  overflow: auto;
}

.no-show {
  display: flex;
  flex-direction: column;
  align-content: center;
  align-items: center;
  justify-content: center;
  height: 70vh;

  img {
    width: 400px;
    height: 400px;
  }

  p {
    color: #606266;
    font-size: 15px;
  }
}
</style>
