<template>
  <div>
    <van-nav-bar v-if="title==='Zadig'"
                 title="Zadig" />
    <van-notice-bar v-if="title==='Zadig'"
                    color="#1989fa"
                    background="#ecf9ff"
                    :scrollable="true"
                    left-icon="info-o">
      欢迎使用 Zadig 移动端，功能有限，如需更多功能请前往 PC 端使用
    </van-notice-bar>
    <router-view />
    <van-tabbar v-model="active"
                active-color="#1989fa"
                inactive-color="#000"
                route>
      <van-tabbar-item replace
                       name="status"
                       to="/mobile/status"
                       icon="browsing-history-o">运行状态</van-tabbar-item>
      <van-tabbar-item replace
                       name="pipelines"
                       to="/mobile/pipelines"
                       icon="play-circle-o">工作流</van-tabbar-item>
      <van-tabbar-item replace
                       name="envs"
                       to="/mobile/envs"
                       icon="points">集成环境</van-tabbar-item>
      <van-tabbar-item replace
                       name="profile"
                       to="/mobile/profile"
                       icon="user-circle-o">用户设置</van-tabbar-item>
    </van-tabbar>

  </div>
</template>

<script>

import { NavBar, Tabbar, TabbarItem, NoticeBar } from 'vant';
export default {
  components: {
    [TabbarItem.name]: TabbarItem,
    [Tabbar.name]: Tabbar,
    [NavBar.name]: NavBar,
    [NoticeBar.name]: NoticeBar
  },

  data() {
    return {
      active: ''
    };
  },

  computed: {
    title() {
      return this.$route.meta.title;
    }

  },
  methods: {
    goBack() {
      this.$router ? this.$router.back() : window.history.back();
    },
  }
};
</script>

<style lang="less">
@import "~@assets/css/component/mobile.less";
</style>