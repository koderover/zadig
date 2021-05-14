<template>
  <div style="height:100%;
    display: flex;
    flex-direction: column;">
    <router-view style="height:100%">
    </router-view>
  </div>

</template>

<script>
import { mapGetters } from 'vuex';
import { getCurrentUserInfoAPI } from '@api';
export default {
  methods: {
    checkLogin() {
      return new Promise((resolve, reject) => {
        getCurrentUserInfoAPI().then(
          response => {
            resolve(true);
          },
          response => {
            reject(false);
          }
        );
      })
    }
  },
  computed: {
    ...mapGetters([
      'signupStatus',
    ])
  },
  created() {
    this.$store.dispatch('getSignupStatus').then(() => {
      if (this.signupStatus.inited) {
        this.checkLogin().then((result) => {
          if (result) {
            if (this.$utils.roleCheck() != null) {
              this.$store.dispatch('getProjectTemplates').then(() => {
              });
            }
          }
        })
      }
      else {
        this.$router.push('/setup');
      }
    })
  }
};
</script>

<style lang="less">
@import url("~@assets/css/common/color.less");
@import url("~@assets/css/common/icon.less");
@import url("~@assets/css/common/font.less");
a {
  text-decoration: none;
}
body {
  font-family: "Overpass", "Noto Sans SC", -apple-system, BlinkMacSystemFont,
    "Helvetica Neue", Helvetica, Segoe UI, Arial, Roboto, "PingFang SC",
    "Hiragino Sans GB", "Microsoft Yahei", sans-serif;
  background-color: #fff;
  height: 100%;
  overflow: hidden;
  overflow-y: auto;
  .el-card {
    background: #fff;
  }
}
</style>
