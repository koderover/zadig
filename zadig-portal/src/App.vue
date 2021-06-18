<template>
  <div style="
    display: flex;
    flex-direction: column;
    height: 100%;">
    <router-view style="height: 100%;">
    </router-view>
  </div>

</template>

<script>
import { mapGetters } from 'vuex'
import { getCurrentUserInfoAPI } from '@api'
export default {
  methods: {
    checkLogin () {
      return new Promise((resolve, reject) => {
        getCurrentUserInfoAPI().then(
          response => {
            resolve(true)
          },
          response => {
            reject(false)
          }
        )
      })
    }
  },
  computed: {
    ...mapGetters([
      'signupStatus'
    ])
  },
  created () {
    this.$store.dispatch('getSignupStatus').then(() => {
      if (this.signupStatus.inited) {
        this.checkLogin().then((result) => {
          if (result) {
            if (this.$utils.roleCheck() != null) {
              this.$store.dispatch('getProjectTemplates')
            }
          }
        })
      } else {
        this.$router.push('/setup')
      }
    })
  }
}
</script>

<style lang="less">
@import url("~@assets/css/common/color.less");
@import url("~@assets/css/common/icon.less");

a {
  text-decoration: none;
}

body {
  height: 100%;
  overflow: hidden;
  overflow-y: auto;
  font-family:
    "Overpass",
    "Noto Sans SC",
    -apple-system,
    BlinkMacSystemFont,
    "Helvetica Neue",
    Helvetica,
    Segoe UI,
    Arial,
    Roboto,
    "PingFang SC",
    "Hiragino Sans GB",
    "Microsoft Yahei",
    sans-serif;
  background-color: #fff;

  .el-card {
    background: #fff;
  }
}
</style>
