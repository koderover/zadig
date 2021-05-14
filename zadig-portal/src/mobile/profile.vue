<template>
  <div class="mobile-profile">
    <van-nav-bar left-arrow
                 @click-left="mobileGoback">
      <template #title>
        {{username}}
      </template>
    </van-nav-bar>
    <div v-if="loading"
         class="load-cover">
      <van-loading type="spinner"
                   color="#1989fa" />
    </div>

    <template v-else>
      <van-panel title="用户名"
                 :desc="username"
                 :status="userRole">
      </van-panel>
      <van-panel title="用户来源"
                 :desc="from">
      </van-panel>
      <van-panel title="最近登录"
                 :desc="$utils.convertTimestamp(currentEditUserInfo.info.lastLogin)">
      </van-panel>
      <van-panel v-if="jwtToken"
                 title="API Token"
                 :desc="jwtToken">
      </van-panel>
      <div class="logout">
        <van-button type="danger"
                    @click="logout"
                    round
                    block>退出登录</van-button>
      </div>

    </template>

  </div>

</template>
<script>
import storejs from '@node_modules/store/dist/store.legacy.js';
import { NavBar, Tag, Panel, Loading, Button, Notify } from 'vant';
import { getCurrentUserInfoAPI, getJwtTokenAPI, userLogoutAPI } from '@api';
export default {
  components: {
    [NavBar.name]: NavBar,
    [Tag.name]: Tag,
    [Panel.name]: Panel,
    [Loading.name]: Loading,
    [Button.name]: Button,
    [Notify.name]: Notify
  },
  data() {
    return {
      loading: true,
      jwtToken: null,
      currentEditUserInfo: {
        "info": {
          "id": 5,
          "name": "",
          "email": "",
          "password": "",
          "phone": "",
          "isAdmin": true,
          "isSuperUser": false,
          "isTeamLeader": false,
          "organization_id": 0,
          "directory": "system",
          "teams": []
        },
        "teams": [],
        "organization": {
          "id": 1,
          "name": "",
          "token": "",
          "website": "",
        }
      },

    }
  },
  methods: {
    getUserInfo() {
      this.loading = true;
      getCurrentUserInfoAPI().then((res) => {
        this.loading = false;
        this.currentEditUserInfo = res;
      });
    },
    getJwtToken() {
      getJwtTokenAPI().then((res) => {
        this.jwtToken = res.token;
      });
    },
    logout() {
      userLogoutAPI().then((res) => {
        storejs.remove('ZADIG_LOGIN_INFO');
        this.$store.dispatch('clearProjectTemplates');
        this.$router.push('/signin');
        Notify({ type: 'success', message: '账号退出成功' });
      });
    },
  },
  computed: {
    title() {
      return this.$route.meta.title;
    },
    username() {
      return this.$store.state.login.userinfo.info.name
    },
    userRole() {
      if (this.currentEditUserInfo.info.isSuperUser) {
        return '管理员';
      }
      else {
        return '普通用户';
      }
    },
    from() {
      return this.currentEditUserInfo.info.directory;
    }
  },
  mounted() {
    this.$store.commit('INJECT_PROFILE', storejs.get('ZADIG_LOGIN_INFO'));
    this.getUserInfo();
    this.getJwtToken();
  },
}
</script>
<style lang="less">
.mobile-profile {
  .load-cover {
    width: 100%;
    height: calc(~"100% - 46px");
    background: rgba(255, 255, 255, 0.5);
    z-index: 999;
    position: fixed;
    left: 0;
    top: 0;
    .van-loading {
      width: 40px;
      height: 40px;
      top: 40%;
      left: 50%;
      transform: translateX(-50%);
    }
  }
  .logout {
    margin-top: 35px;
    padding: 0px 12px;
  }
}
</style>