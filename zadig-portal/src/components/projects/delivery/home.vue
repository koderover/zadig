<template>
  <div class="insight-home"
       v-loading="loading"
       element-loading-text="加载中..."
       element-loading-spinner="iconfont iconfont-loading iconjiaofu">
    <div class="no-show"
         v-if="loading || productNames.length===0">
      <img src="@assets/icons/illustration/version_manage.svg"
           alt="" />
      <p v-if="!loading">没有版本管理信息</p>
    </div>
    <router-view></router-view>
  </div>
</template>
<script>
import { getVersionProductListAPI } from '@api'
import bus from '@utils/event_bus'
export default {
  data () {
    return {
      loading: true,
      productNames: []
    }
  },
  computed: {
    currentOrganizationId () {
      return this.$store.state.login.userinfo.organization.id
    }
  },
  methods: {
    getVersionProductList () {
      const orgId = this.currentOrganizationId
      getVersionProductListAPI(orgId).then((res) => {
        this.loading = false
        this.productNames = res
        if (res.length === 0) {
          return
        } else {
          const routerList = res.map(re => {
            return {
              name: re,
              url: `/v1/delivery/version/${re}`
            }
          })
          bus.$emit(`set-sub-sidebar-title`, {
            title: '项目列表',
            routerList: routerList
          })
        }
        if (!this.$route.params.project_name) {
          this.$router.replace(`/v1/delivery/version/${res[0]}`)
        }
      }).catch(() => {
        this.$message.error('获取项目信息出错！')
        this.$router.go(-1)
      })
    }
  },
  watch: {
    $route (to, from) {
      if (!to.params.project_name && this.productNames.length > 0) {
        this.$router.replace(`/v1/delivery/version/${this.productNames[0]}`)
      }
    }
  },
  created () {
    this.getVersionProductList()
    bus.$emit(`set-topbar-title`, { title: '版本管理', breadcrumb: [] })
    bus.$emit(`show-sidebar`, true)
    bus.$emit(`sub-sidebar-opened`, false)
  }
}
</script>

<style lang="less" >
.insight-home {
  position: relative;
  flex: 1;
  overflow: auto;

  .no-show {
    display: flex;
    flex-direction: column;
    align-content: center;
    align-items: center;
    justify-content: center;
    height: 70vh;
    margin: 15px 30px;

    img {
      width: 400px;
      height: 400px;
    }

    p {
      color: #606266;
      font-size: 15px;
    }
  }
}
</style>
