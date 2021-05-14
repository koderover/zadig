<template>
  <div class="onborading-main-container">
    <div class="main-view large-sidebar">
      <div class="side-bar-container"
           :style="{width: sideWide? '196px':'66px'}">
        <sidebar class="side-bar-component"
                 @sidebar-width="sideWide = $event"></sidebar>
        <subSidebar class="cf-sub-side-bar"></subSidebar>
      </div>
      <div class="content-wrap">
        <topbar></topbar>
        <router-view>
        </router-view>
      </div>
    </div>
  </div>

</template>

<script>
import { mapGetters } from 'vuex';
import { getCurrentUserInfoAPI } from '@api';
import sidebar from './home/sidebar.vue';
import subSidebar from './home/sub_sidebar.vue';
import topbar from './home/topbar.vue';
export default {
  data() {
    return {
      sideWide: true,
    };
  },
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
    ]),
  },
  components: {
    sidebar,
    topbar,
    subSidebar,
  },
  created() {
    this.$store.dispatch('getSignupStatus').then(() => {
      this.checkLogin().then((result) => {
        if (result) {
          if (this.$utils.roleCheck() != null) {
            this.$store.dispatch('getProjectTemplates').then(() => {
            });
          }
        }
      })
    })
  }
};
</script>

<style lang="less">
a {
  text-decoration: none;
}
button {
  &:focus {
    outline: none;
  }
}
body {
  background-color: #fff;
  height: 100%;
  overflow: hidden;
  overflow-y: auto;
  .el-card {
    background: #fff;
  }
  .onborading-main-container {
    min-height: 100vh;
    width: 100vw;
    min-width: 768px;
    height: 100vh;
    .main-view {
      > * {
        -webkit-box-flex: 0;
        -ms-flex: 0 0 auto;
        flex: 0 0 auto;
      }
      height: 100%;
      overflow: hidden;
      display: flex;
      align-items: stretch;
      justify-content: flex-start;
      .side-bar-container {
        position: relative;
        transition: width 350ms;
        background-color: #f5f7fa;
        .side-bar-component {
          position: absolute;
          z-index: 2;
          height: 100%;
          display: flex;
          align-items: center;
          justify-content: stretch;
          -webkit-box-orient: vertical;
          -webkit-box-direction: normal;
          -ms-flex-direction: column;
          flex-direction: column;
        }
        .cf-sub-side-bar {
          position: absolute;
          left: 66px;
          z-index: 1;
          height: 100%;
          display: flex;
          align-items: center;
          justify-content: stretch;
          -webkit-box-orient: vertical;
          flex-direction: column;
        }
      }
      .content-wrap {
        flex-grow: 1;
        flex-shrink: 1;
        overflow: auto;
        position: relative;
        height: 100%;
        display: flex;
        align-items: stretch;
        justify-content: flex-start;
        -webkit-box-orient: vertical;
        -webkit-box-direction: normal;
        -ms-flex-direction: column;
        flex-direction: column;
      }
    }
  }
  .el-message-box__wrapper {
    .el-message-box {
      .el-message-box__header {
        .el-message-box__title {
          padding-right: 26px;
          line-height: 1.4;
        }
      }
    }
  }
}
</style>
