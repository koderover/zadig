<template>
  <div @mouseenter="enterSidebar"
       @mouseleave="leaveSidebar"
       class="cf-side-bar full-h"
       :class="{ 'small-sidebar': !showSidebar }">
    <a @click="changeSidebar"
       class="sidebar-size-toggler">
      <i :class="showSidebar?'el-icon-arrow-right':'el-icon-arrow-left'"></i>
    </a>

    <div class="cf-side-bar-header">
      <router-link to="/v1/status">
        <img v-if="showSidebar&&!showBackPath"
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

          <div class="nav grow-nothing ">
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
import bus from '@utils/event_bus'
import _ from 'lodash'
import { mapGetters } from 'vuex'
export default {
  data () {
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
        }
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
          },
          {
            name: '构建镜像管理',
            icon: 'iconfont iconjingxiang',
            url: 'system/imgs'
          }]
        },
        {
          category_name: '基础组件',
          items: [{
            name: '镜像仓库',
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
            subItems: [{
              name: '版本管理',
              url: 'delivery/version',
              icon: 'iconfont iconbanben'
            },
            {
              name: '交付物追踪',
              url: 'delivery/artifacts',
              icon: 'iconfont iconbaoguanli'
            }]
          }]
        },
        {
          category_name: '质量管理',
          items: [
            {
              name: '测试管理',
              icon: 'iconfont icontest',
              url: 'tests'
            }
          ]
        },
        {
          category_name: '设置',
          items: [
            {
              name: '用户设置',
              icon: 'iconfont iconfenzucopy',
              url: 'profile'
            },
            {
              name: '系统设置',
              icon: 'iconfont iconicon_jichengguanli',
              url: 'system'
            }
          ]
        }
      ]
    }
  },
  methods: {
    enterSidebar: _.debounce(function () {
      if (this.subSidebarOpened) {
        this.showSidebar = true
      }
    }, 300),
    leaveSidebar: _.debounce(function () {
      if (this.subSidebarOpened) {
        this.showSidebar = false
      }
    }, 100),
    changeSidebar () {
      this.showSidebar = !this.showSidebar
      this.$emit('sidebar-width', this.showSidebar || this.subSideBar)
    },
    collapseMenu (nav) {
      nav.isOpened = !nav.isOpened
    }
  },
  computed: {
    ...mapGetters([
      'signupStatus'
    ]),
    navList () {
      const path = this.$route.path
      if (path.includes('/v1/enterprise')) {
        return this.enterpriseMenu
      } else if (path.includes('/v1/system')) {
        return this.systemMenu
      } else {
        return this.defaultMenu
      }
    },
    showBackPath () {
      const path = this.$route.path
      if (path.includes('/v1/enterprise')) {
        this.backTitle = '用户管理'
        return true
      } else if (path.includes('/v1/system')) {
        this.backTitle = '系统设置'
        return true
      } else {
        return false
      }
    }
  },
  watch: {
    showSidebar: function (new_val, old_val) {
      this.$store.commit('SET_SIDEBAR_STATUS', new_val)
    }
  },
  created () {
    bus.$on('show-sidebar', (params) => {
      this.showSidebar = params
    })
    bus.$on('sub-sidebar-opened', (params) => {
      this.subSidebarOpened = params
    })
    bus.$on('sub-sidebar-opened', (params) => {
      this.subSideBar = params
      this.showSidebar = !params
      this.$emit('sidebar-width', true)
    })
  }
}
</script>

<style lang="less">
.cf-side-bar {
  position: relative;
  display: -webkit-box;
  display: -ms-flexbox;
  display: flex;
  -ms-flex-direction: column;
  flex-direction: column;
  align-items: left;
  justify-content: space-between;
  width: 196px;
  margin-right: 0;
  font-size: 12px;
  background-color: #f5f7fa;
  border-right: 1px solid #e6e9f0;
  transition: width 350ms, margin-width 230ms;
  -webkit-box-align: left;
  -ms-flex-align: left;
  -webkit-box-pack: justify;
  -ms-flex-pack: justify;
  -webkit-box-orient: vertical;
  -webkit-box-direction: normal;

  .sidebar-size-toggler {
    position: absolute;
    right: 0;
    bottom: 10px;
    z-index: 1;
    display: block;
    width: 33px;
    height: 30px;
    color: #fff;
    line-height: 30px;
    text-align: center;
    text-decoration: none;
    background-color: #c0c4cc;
    border-radius: 10px 0 0 10px;
    cursor: pointer;

    i {
      display: inline-block;
      transform: rotateZ(180deg);
    }
  }

  h4 {
    &.category-name {
      display: inline-block;
      width: 100%;
      margin-top: 0;
      margin-bottom: 10px;
      padding-left: 20px;
      color: #000;
      font-weight: bold;
      font-size: 14px;
      text-align: left;
      text-transform: uppercase;

      i {
        position: relative;
        top: -1px;
        display: inline-block;
        font-size: 9px;
      }

      .new-feature {
        padding: 1px 3px;
        color: #e02711;
        font-weight: 300;
        font-size: 11px;
        border: 1px solid #e02711;
        border-radius: 4px;
      }
    }
  }

  .nav {
    flex-basis: 0;
    width: 100%;
    margin-bottom: 0;
    padding-left: 0;
    overflow: auto;
    overflow-x: hidden;
    list-style: none;
    -ms-flex-preferred-size: 0;

    &.grow-all {
      flex-grow: 1;
      -webkit-box-flex: 1;
      -ms-flex-positive: 1;
    }

    &.grow-nothing {
      flex-grow: 0;
      -webkit-box-flex: 0;
      -ms-flex-positive: 0;
    }
  }

  .category-wrapper {
    position: relative;
    -ms-flex: none !important;
    flex: none !important;
    padding: 20px 0 10px;
    -webkit-box-flex: 0 !important;

    &:last-child {
      padding-bottom: 30px;
    }

    &.divider {
      &::before {
        position: absolute;
        top: 0;
        right: 0;
        left: 4px;
        width: 30px;
        height: 1px;
        margin: auto;
        background-color: #c0c4cc;
        content: "";
      }
    }
  }

  .sub-menu {
    position: relative;
    margin: 0;
    padding-left: 0;
    list-style: none;
    background-color: #fff;
    border-right: solid 1px #e6e6e6;

    .sub-menu-item {
      position: relative;
      box-sizing: border-box;
      min-width: 200px;
      height: 38px;
      padding: 0 35px;
      color: #303133;
      font-size: 14px;
      line-height: 38px;
      white-space: nowrap;
      list-style: none;
      cursor: pointer;
      transition: border-color 0.3s, background-color 0.3s, color 0.3s;

      &:hover {
        background-color: #e1edfa;
      }

      .sub-item-icon {
        display: inline-block;
        width: 35px;
        text-align: center;

        i {
          color: #1989fa;
          font-size: 19px;
        }
      }

      .sub-item-label {
        padding-left: 0;
        color: #434548;
        font-size: 13px;
        line-height: 2.2;
        text-align: left;
      }
    }

    .sub-menu-item-group > ul {
      padding: 0;
    }
  }

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
        padding: 0;

        .sub-item-icon {
          width: 65px;
        }
      }
    }
  }

  .sub-menu-item-group {
    .active {
      .sub-menu-item {
        background-color: #e1edfa;
      }
    }
  }

  .cf-side-bar-header__icon {
    float: left;
    color: #1989fa;
    font-weight: normal;
    font-size: 16px;
  }

  .cf-side-bar-header__info {
    padding: 20px 0;

    .logo-title {
      color: #434548;
      font-weight: bold;
      font-size: 16px;
      line-height: 16px;
      text-transform: uppercase;
    }

    .logo-title_subtitle {
      max-width: 160px;
      margin-top: 5px;
      overflow: hidden;
      color: #54a2f3;
      font-size: 12px;
      line-height: 16px;
      white-space: nowrap;
      text-align: left;
      text-overflow: ellipsis;
    }
  }

  .cf-side-bar-header {
    display: flex;
    justify-content: center;
    width: 100%;
    margin-top: 10px;
    margin-bottom: 10px;

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
        margin: 0 auto;
        padding: 0;
      }
    }
  }

  .nav-item-icon {
    display: -webkit-box;
    display: -ms-flexbox;
    display: flex;
    flex-grow: 0;
    align-items: center;
    justify-content: center;
    width: 62px;
    max-width: 62px;
    margin: 0;
    padding: 0;
    color: #1989fa;
    font-size: 22px;
    text-align: center;
    -webkit-box-flex: 0;
    -ms-flex-positive: 0;
    -webkit-box-align: center;
    -ms-flex-align: center;
    -webkit-box-pack: center;
    -ms-flex-pack: center;

    .iconfont {
      font-size: 22px;
    }
  }

  .nav-item {
    display: -webkit-box;
    display: -ms-flexbox;
    display: flex;
    align-items: center;
    justify-content: flex-start;
    width: 100%;
    min-width: 246px;
    padding: 4px 0 4px 0;
    overflow: hidden;
    border-left: 4px solid #f5f7fa;
    outline: none;
    -webkit-box-align: center;
    -ms-flex-align: center;
    -webkit-box-pack: start;
    -ms-flex-pack: start;

    .nav-item-label {
      display: -webkit-box;
      display: -ms-flexbox;
      display: flex;
      align-items: center;
      min-width: 130px;
      max-width: 186px;
      padding-right: 37px;
      padding-left: 0;
      color: #434548;
      font-size: 14px;
      line-height: 2.2;
      white-space: nowrap;
      text-align: left;
      -webkit-transition: color 200ms ease-in;
      transition: color 200ms ease-in;
      -webkit-box-align: center;
      -ms-flex-align: center;

      .arrow {
        position: static;
        margin-top: -3px;
        margin-left: 8px;
        vertical-align: middle;
      }
    }
  }

  .nav-item.active,
  .nav-item:hover {
    padding-left: 0;
    background-color: #e1edfa;
    border-left: 4px solid #1989fa;
  }

  .category-wrapper:first-child {
    margin-top: 0;
  }

  .nav__new-wrapper {
    position: relative;
  }

  .nav-item-bottom {
    margin-top: auto;
  }
}
</style>
