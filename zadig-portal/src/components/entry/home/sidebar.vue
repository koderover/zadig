<template>
  <div v-on:mouseenter="enterSidebar"
       v-on:mouseleave="leaveSidebar"
       class="cf-side-bar full-h"
       :class="{ 'small-sidebar': !showSidebar }">
    <a @click="changeSidebar"
       class="sidebar-size-toggler">
      <i :class="showSidebar?'el-icon-arrow-right':'el-icon-arrow-left'"></i>
    </a>

    <div class="cf-side-bar-header">
      <router-link to="/v1/status">
        <img v-if="showSidebar&&!showBackPath&&logoUrl"
             class="logo"
             :src="logoUrl">
        <img v-if="showSidebar&&!showBackPath&&!logoUrl"
             class="logo"
             src="@assets/icons/logo/default-logo.png">
        <img v-if="!showSidebar&&!showBackPath"
             class="logo"
             :class="{'small':!showSidebar}"
             src="@assets/icons/logo/small-logo.png">
      </router-link>
      <router-link class="cf-side-bar-header back-to"
                   v-if="showSidebar&&showBackPath"
                   :to="backUrl">
        <div class="cf-side-bar-header__icon">
          <i class="icon el-icon-back"></i>
        </div>
        <div class="cf-side-bar-header__info">
          <div class="logo-title">{{backTitle}}</div>
          <div class="logo-title logo-title_subtitle">返回上一层</div>
        </div>
      </router-link>
      <router-link class="cf-side-bar-header"
                   v-if="!showSidebar&&showBackPath"
                   :to="backUrl">
        <div class="cf-side-bar-header__icon">
          <i class="icon el-icon-back"></i>
        </div>
      </router-link>
    </div>
    <div class="nav grow-all main-menu cf-sidebar-scroll">

      <div v-for="(item,index) in navList"
           :key="index"
           class="category-wrapper"
           :class="{'divider':!showSidebar}">
        <h4 v-show="showSidebar"
            class="category-name">
          {{item.category_name}}
          <span v-if="item.new_feature"
                class="new-feature">New</span>
        </h4>
        <div class="nav__new-wrapper"
             v-for="(nav,nav_index) in navList[index].items"
             :key="nav_index">

          <div v-if="ifShowItem(nav)"
               class="nav grow-nothing ">
            <div @click="collapseMenu(nav)"
                 v-if="nav.hasSubItem"
                 class="nav-item">
              <div class="nav-item-icon">
                <i :class="nav.icon"></i>
              </div>
              <a href="javascript:void(0)">
                <div class="nav-item-label">
                  {{nav.name}}
                  <i v-if="nav.isOpened"
                     class="el-icon-arrow-up arrow"></i>
                  <i v-else-if="!nav.isOpened"
                     class="el-icon-arrow-down arrow"></i>
                </div>
              </a>
            </div>
            <router-link v-else
                         class="nav-item"
                         active-class="active"
                         :to="`/v1/${nav.url}`">
              <div class="nav-item-icon">
                <i :class="nav.icon"></i>
              </div>
              <div class="nav-item-label">
                {{nav.name}}
              </div>
            </router-link>
            <ul v-if="nav.hasSubItem && nav.isOpened"
                class="sub-menu"
                style="overflow: hidden;">
              <li class="sub-menu-item-group">
                <ul>
                  <router-link v-for="(subItem,index) in nav.subItems"
                               :key="index"
                               active-class="active"
                               :to="`/v1/${subItem.url}`">
                    <li class="sub-menu-item">
                      <span class="sub-item-icon">
                        <i :class="subItem.icon"></i>
                      </span>
                      <span class="sub-item-label">
                        {{subItem.name}}
                      </span>
                    </li>
                  </router-link>
                </ul>
              </li>
            </ul>

          </div>

        </div>

        <div class="nav__new-wrapper">
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import bus from '@utils/event_bus';
import _ from 'lodash';
import { mapGetters } from 'vuex';
export default {
  data() {
    return {
      showSidebar: true,
      subSideBar: false,
      subSidebarOpened: false,
      backTitle: '',
      backUrl: '/v1/status',
      enterpriseMenu: [
        {
          category_name: '用户管理',
          items: [
            {
              name: '用户管理',
              icon: 'iconfont icongeren',
              url: 'enterprise/users/manage'
            }]
        },
      ],
      systemMenu: [
        {
          category_name: '第三方集成',
          items: [{
            name: '集成管理',
            icon: 'iconfont iconicon_jichengguanli',
            url: 'system/integration'
          },
          {
            name: '应用设置',
            icon: 'iconfont iconyingyongshezhi',
            url: 'system/apps'
          }]
        },
        {
          category_name: '基础组件',
          items: [{
            name: 'REGISTRY 管理',
            icon: 'iconfont icondocker',
            url: 'system/registry'
          },
          {
            name: '对象存储',
            icon: 'iconfont iconduixiangcunchu',
            url: 'system/storage'
          }]
        },
        {
          category_name: '系统配置',
          items: [{
            name: '系统配置',
            icon: 'iconfont iconfuwupeizhi',
            url: 'system/config'
          }]
        }
      ],
      defaultMenu: [
        {
          category_name: '产品交付',
          items: [{
            name: '运行状态',
            icon: 'iconfont iconyunhangzhuangtai',
            url: 'status'
          },
          {
            name: '项目',
            icon: 'iconfont iconxiangmu',
            url: 'projects'
          },
          {
            name: '工作流',
            icon: 'iconfont icongongzuoliucheng',
            url: 'pipelines'
          },
          {
            name: '集成环境',
            icon: 'iconfont iconrongqi',
            url: 'envs'
          },
          {
            name: '交付中心',
            icon: 'iconfont iconjiaofu',
            hasSubItem: true,
            isOpened: false,
            subItems: [
              {
                name: '交付物追踪',
                url: 'delivery/artifacts',
                icon: 'iconfont iconbaoguanli'
              }],
          }]
        },
        {
          category_name: '设置',
          items: [
            {
              name: '用户设置',
              icon: 'iconfont iconfenzucopy',
              url: 'profile'
            }
          ]
        }
      ],
      adminSettingItems: [
        {
          name: '系统设置',
          icon: 'iconfont iconicon_jichengguanli',
          url: 'system'
        },
      ],
    }
  },
  methods: {
    ifShowItem(nav) {
      let status = true
      if (nav.features) {
        status = this.signupStatus && this.signupStatus.features && this.signupStatus.features.includes(nav.features)
      }
      return status
    },
    enterSidebar: _.debounce(function () {
      if (this.subSidebarOpened) {
        this.showSidebar = true;
      }
    }, 300),
    leaveSidebar: _.debounce(function () {
      if (this.subSidebarOpened) {
        this.showSidebar = false;
      }
    }, 100),
    changeSidebar() {
      this.showSidebar = !this.showSidebar;
      this.$emit('sidebar-width', this.showSidebar || this.subSideBar);
    },
    collapseMenu(nav) {
      nav.isOpened = !nav.isOpened;
    },
    checkShowAdminSetting() {
      if (this.$utils.roleCheck().superAdmin) {
        let setting = this.defaultMenu[this.defaultMenu.length - 1];
        if (setting.category_name === '设置') {
          setting.items = [].concat(this.adminSettingItems, setting.items);
        }
      }
    }
  },
  computed: {
    ...mapGetters([
      'signupStatus',
    ]),
    navList() {
      const path = this.$route.path;
      if (path.includes('/v1/enterprise')) {
        if (this.showAnalytics) {
          return this.enterpriseMenu;
        }
        else {
          return this.enterpriseMenu.filter(element => {
            return element.category_name !== '统计分析';
          });
        }
      }
      else if (path.includes('/v1/system')) {
        return this.systemMenu;
      }
      else {
        if (this.showInsight && this.showBoard) {
          this.defaultMenu
          return this.defaultMenu;
        }
        else if (this.showInsight && !this.showBoard) {
          this.defaultMenu.forEach(element => {
            if (element.category_name === '质效中心') {
              element.items = element.items.filter(item => {
                return item.name !== 'DevOps 洞察';
              });
            }
          });
          return this.defaultMenu;
        }
        else if (!this.showInsight && this.showBoard) {
          this.defaultMenu.forEach(element => {
            if (element.category_name === '质效中心') {
              element.items = element.items.filter(item => {
                return item.name !== '质效看板';
              });
            }
          });
          return this.defaultMenu;
        }
        else if (!this.showInsight && !this.showBoard) {
          return this.defaultMenu.filter(element => {
            return element.category_name !== '质效中心';
          });
        }
      }
    },
    showBackPath() {
      const path = this.$route.path;
      if (path.includes('/v1/enterprise')) {
        this.backTitle = '用户管理';
        return true;
      }
      else if (path.includes('/v1/system')) {
        this.backTitle = '系统设置';
        return true;
      }
    },
    logoUrl: {
      get: function () {
        if (this.signupStatus && this.signupStatus.logo) {
          return this.signupStatus.logo
        }
        else {
          return '';
        }
      },
      set: function (newValue) {
      }
    },
    showInsight: {
      get: function () {
        if (this.signupStatus && this.signupStatus.features && this.signupStatus.features.length > 0) {
          if (this.signupStatus.features.includes('insight')) {
            return true;
          }
          else {
            return false;
          }
        }
      },
      set: function (newValue) {
      }
    },
    showBoard: {
      get: function () {
        if (this.signupStatus && this.signupStatus.features && this.signupStatus.features.length > 0) {
          if (this.signupStatus.features.includes('board')) {
            return true;
          }
          else {
            return false;
          }
        }
      },
      set: function (newValue) {
      }
    },
    showAnalytics: {
      get: function () {
        if (this.signupStatus && this.signupStatus.features && this.signupStatus.features.length > 0) {
          if (this.signupStatus.features.includes('koderover')) {
            return true;
          }
          else {
            return false;
          }
        }
      },
      set: function (newValue) {
      }
    },
  },
  watch: {
    'showSidebar': function (new_val, old_val) {
      this.$store.commit('SET_SIDEBAR_STATUS', new_val);
    }
  },
  created() {
    bus.$on('show-sidebar', (params) => {
      this.showSidebar = params;
    });
    bus.$on('sub-sidebar-opened', (params) => {
      this.subSidebarOpened = params;
    });
    bus.$on('sub-sidebar-opened', (params) => {
      this.subSideBar = params;
      this.showSidebar = !params;
      this.$emit('sidebar-width', true);
    });
    this.checkShowAdminSetting();
  }
}
</script>

<style lang="less">
.cf-side-bar {
  @import url("~@assets/css/common/scroll-bar.less");
  width: 196px;
  position: relative;
  background-color: #f5f7fa;
  border-right: 1px solid #e6e9f0;
  font-size: 12px;
  transition: width 350ms, margin-width 230ms;
  margin-right: 0;
  display: -webkit-box;
  display: -ms-flexbox;
  display: flex;
  -webkit-box-align: left;
  -ms-flex-align: left;
  align-items: left;
  -webkit-box-pack: justify;
  -ms-flex-pack: justify;
  justify-content: space-between;
  -webkit-box-orient: vertical;
  -webkit-box-direction: normal;
  -ms-flex-direction: column;
  flex-direction: column;

  &.small-sidebar {
    width: 66px;
    .category-wrapper {
      max-width: 66px;
      padding: 25px 0 30px;
    }
    .nav {
      &.main-menu {
        margin-top: 25px;
      }
    }
    .sub-menu {
      .sub-menu-item {
        padding: 0px;
        .sub-item-icon {
          width: 65px;
        }
      }
    }
  }
  .main-menu {
  }
  .sub-menu {
    border-right: solid 1px #e6e6e6;
    list-style: none;
    position: relative;
    margin: 0;
    padding-left: 0;
    background-color: #fff;
    .sub-menu-item-group > ul {
      padding: 0;
    }
    .sub-menu-item-group {
      .active {
        .sub-menu-item {
          background-color: #e1edfa;
        }
      }
    }
    .sub-menu-item {
      font-size: 14px;
      color: #303133;
      list-style: none;
      cursor: pointer;
      position: relative;
      transition: border-color 0.3s, background-color 0.3s, color 0.3s;
      box-sizing: border-box;
      white-space: nowrap;
      height: 38px;
      line-height: 38px;
      padding: 0 35px;
      min-width: 200px;
      &:hover {
        background-color: #e1edfa;
      }
      .sub-item-icon {
        width: 35px;
        display: inline-block;
        text-align: center;
        i {
          color: #1989fa;
          font-size: 19px;
        }
      }
      .sub-item-label {
        font-size: 13px;
        line-height: 2.2;
        text-align: left;
        color: #434548;
        padding-left: 0;
      }
    }
  }

  .sidebar-size-toggler {
    width: 33px;
    height: 30px;
    line-height: 30px;
    display: block;
    position: absolute;
    right: 0;
    bottom: 10px;
    background-color: #c0c4cc;
    color: #fff;
    text-decoration: none;
    text-align: center;
    cursor: pointer;
    z-index: 1;
    border-radius: 10px 0 0 10px;
    i {
      display: inline-block;
      transform: rotateZ(180deg);
    }
    &:hover {
    }
  }
  .cf-side-bar-header__icon {
    font-size: 16px;
    font-weight: normal;
    color: #1989fa;
    float: left;
  }
  .cf-side-bar-header__info {
    padding: 20px 0;
    .logo-title {
      text-transform: uppercase;
      color: #434548;
      font-size: 16px;
      font-weight: bold;
      line-height: 16px;
    }
    .logo-title_subtitle {
      max-width: 160px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: 12px;
      color: #54a2f3;
      text-align: left;
      margin-top: 5px;
      line-height: 16px;
    }
  }
  .cf-side-bar-header {
    width: 100%;
    display: flex;
    margin-bottom: 10px;
    margin-top: 10px;
    justify-content: center;
    &.back-to {
      justify-content: flex-start;
      margin-left: 15px;
      .cf-side-bar-header__info {
        padding: 0;
      }
      .cf-side-bar-header__icon {
        width: 35px;
      }
    }
    .logo {
      height: 50px;
      background-size: cover;
      &.small {
        width: 50px;
        height: 50px;
        padding: 0px;
        margin: 0 auto;
      }
    }
  }
  .nav-item.active,
  .nav-item:hover {
    border-left: 4px solid #1989fa;
    padding-left: 0;
    background-color: #e1edfa;
  }
  .nav {
    padding-left: 0;
    margin-bottom: 0;
    list-style: none;
  }
  .nav-item-icon {
    -webkit-box-flex: 0;
    -ms-flex-positive: 0;
    flex-grow: 0;
    text-align: center;
    padding: 0px;
    font-size: 22px;
    margin: 0;
    width: 62px;
    max-width: 62px;
    color: #1989fa;
    display: -webkit-box;
    display: -ms-flexbox;
    display: flex;
    -webkit-box-align: center;
    -ms-flex-align: center;
    align-items: center;
    -webkit-box-pack: center;
    -ms-flex-pack: center;
    justify-content: center;
    .iconfont {
      font-size: 22px;
    }
  }
  .nav-item {
    display: -webkit-box;
    display: -ms-flexbox;
    display: flex;
    -webkit-box-align: center;
    -ms-flex-align: center;
    align-items: center;
    -webkit-box-pack: start;
    -ms-flex-pack: start;
    justify-content: flex-start;
    width: 100%;
    outline: none;
    min-width: 246px;
    overflow: hidden;
    padding: 4px 0 4px 0;
    border-left: 4px solid #f5f7fa;
    .nav-item-label {
      font-size: 14px;
      line-height: 2.2;
      text-align: left;
      color: #434548;
      min-width: 130px;
      max-width: 186px;
      padding-left: 0;
      -webkit-transition: color 200ms ease-in;
      transition: color 200ms ease-in;
      padding-right: 37px;
      white-space: nowrap;
      display: -webkit-box;
      display: -ms-flexbox;
      display: flex;
      -webkit-box-align: center;
      -ms-flex-align: center;
      align-items: center;
      .arrow {
        position: static;
        vertical-align: middle;
        margin-left: 8px;
        margin-top: -3px;
      }
    }
  }
  .nav {
    width: 100%;
    overflow: auto;
    overflow-x: hidden;
    -ms-flex-preferred-size: 0;
    flex-basis: 0;
    &.grow-all {
      -webkit-box-flex: 1;
      -ms-flex-positive: 1;
      flex-grow: 1;
    }
    &.grow-nothing {
      -webkit-box-flex: 0;
      -ms-flex-positive: 0;
      flex-grow: 0;
    }
  }
  .category-wrapper {
    position: relative;
    -webkit-box-flex: 0 !important;
    -ms-flex: none !important;
    flex: none !important;
    padding: 20px 0 10px;
    &:last-child {
      padding-bottom: 30px;
    }
    &.divider {
      &:before {
        content: "";
        position: absolute;
        top: 0;
        left: 4px;
        right: 0;
        margin: auto;
        width: 30px;
        height: 1px;
        background-color: #c0c4cc;
      }
    }
  }
  .category-wrapper:first-child {
    margin-top: 0px;
  }
  h4 {
    &.category-name {
      color: #000000;
      font-size: 14px;
      font-weight: bold;
      text-transform: uppercase;
      text-align: left;
      padding-left: 20px;
      margin-top: 0;
      margin-bottom: 10px;
      display: inline-block;
      width: 100%;
      i {
        position: relative;
        top: -1px;
        display: inline-block;
        font-size: 9px;
      }
      .new-feature {
        color: #e02711;
        font-size: 11px;
        border: 1px solid #e02711;
        border-radius: 4px;
        padding: 1px 3px;
        font-weight: 300;
      }
    }
  }
  .nav__new-wrapper {
    position: relative;
  }
  .nav-item-bottom {
    margin-top: auto;
  }
}
</style>