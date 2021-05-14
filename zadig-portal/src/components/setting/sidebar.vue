<template>
  <div>
    <div class="sidebar-wrapper"
         style="flex: 0 0 0 80px; order: -1;">
      <div class="sidebar"
           :class="{ sidebarCollapse: !showSidebar }">
        <div class="logo-wrapper">
        </div>
        <div class="header">
          <span class="title"><i class="iconfont iconshezhi"></i> {{showSidebar?'系统设置':'设置'}}</span>
        </div>
        <div class="sidebar-nav-list-divider">
          <div class="divider"
               :class="{ sidebarCollapseDivider: !showSidebar }"></div>
        </div>
        <div class="sidebar-nav-lists">
          <ul class="sidebar-nav-list">
              <router-link to="/setting/integration"
                           active-class="activeTab">
                <li class="sidebar-item"
                    :class="{ sidebarCollapseItem: !showSidebar}">
                  <span class="view-name"><i class="iconfont iconicon_jichengguanli"></i>
                    {{showSidebar?'集成管理':'集成'}}</span>
                  <span v-if="showSidebar"
                        class="el-icon-arrow-right next"></span>
                </li>
              </router-link>
              <router-link to="/setting/apps"
                           active-class="activeTab">
                <li class="sidebar-item"
                    :class="{ sidebarCollapseItem: !showSidebar}">
                  <span class="view-name"><i class="iconfont iconyingyongshezhi"></i>
                    {{showSidebar?'应用设置':'应用'}}</span>
                  <span v-if="showSidebar"
                        class="el-icon-arrow-right next"></span>
                </li>
              </router-link>
              <router-link to="/setting/registry"
                           active-class="activeTab">
                <li class="sidebar-item"
                    :class="{ sidebarCollapseItem: !showSidebar}">
                  <span class="view-name"><i class="iconfont icondocker"></i>
                    {{showSidebar?'Registry 管理':'Reg'}}</span>
                  <span v-if="showSidebar"
                        class="el-icon-arrow-right next"></span>
                </li>
              </router-link>
              <router-link to="/setting/storage"
                           active-class="activeTab">
                <li class="sidebar-item"
                    :class="{ sidebarCollapseItem: !showSidebar}">
                  <span class="view-name"><i class="iconfont iconduixiangcunchu"></i>
                    {{showSidebar?'对象存储':'存储'}}</span>
                  <span v-if="showSidebar"
                        class="el-icon-arrow-right next"></span>
                </li>
              </router-link>
              <router-link to="/setting/cluster"
                           active-class="activeTab">
                <li v-if="showClusterManage"
                    class="sidebar-item"
                    :class="{ sidebarCollapseItem: !showSidebar}">
                  <span class="view-name"><i class="iconfont iconjiqun"></i>
                    {{showSidebar?'集群管理':'集群'}}</span>
                  <span v-if="showSidebar"
                        class="el-icon-arrow-right next"></span>
                </li>
              </router-link>
          </ul>
        </div>
        <div class="sidebar-collapse">
          <div class="sidebar-collapse-item">
            <span v-if="showSidebar"
                  @click="showSidebar = !showSidebar"
                  class="el-icon-d-arrow-left"></span>
            <span v-if="showSidebar"
                  @click="showSidebar = !showSidebar"
                  class="collapse-label">收缩侧边栏</span>
            <span v-if="!showSidebar"
                  @click="showSidebar = !showSidebar"
                  class="el-icon-d-arrow-right"></span>
          </div>
        </div>
      </div>
    </div>

  </div>

</template>

<script type="text/javascript">
import { mapGetters } from 'vuex';
export default {
  data() {
    return {
      showSidebar: true
    };
  },
  methods: {},
  computed: {
    ...mapGetters([
      'signupStatus',
    ]),
    showClusterManage: {
      get: function () {
        if (this.signupStatus && this.signupStatus.features && this.signupStatus.features.length > 0) {
          if (this.signupStatus.features.includes('cluster_manager')) {
            return true;
          }
          else {
            return false;
          }
        }
      },
      set: function (newValue) {
      }
    }
  },
};
</script>

<style lang="less">
.sidebar-wrapper {
  position: relative;
  height: 100%;
  display: flex;
  .sidebar {
    transition: all 0.15s ease-in-out;
    display: flex;
    flex-direction: column;
    width: 230px;
    background-color: #f5f7fa;
    border-right: 1px solid #e6e9f0;
    .logo-wrapper {
      padding-top: 20px;
    }
    .header {
      .title {
        font-size: 15px;
      }
      vertical-align: middle;
      text-align: center;
    }
    .sidebar-nav-list-divider {
      margin: 10px 0;
      .divider {
        transition: all 0.15s ease-in-out;
        margin: 0 auto;
        height: 1px;
        width: 180px;
        background-color: #e6e9f0;
      }
      .sidebarCollapseDivider {
        transition: all 0.15s ease-in-out;
        width: 60px;
      }
    }
    .sidebar-nav-lists {
      overflow: hidden;
      flex-grow: 1;
      display: flex;
      flex-direction: column;
      &::-webkit-scrollbar-track {
        border-radius: 6px;
        background-color: #f5f5f5;
      }
      &::-webkit-scrollbar {
        width: 6px;
        background-color: #f5f5f5;
      }
      &::-webkit-scrollbar-thumb {
        border-radius: 6px;
        background-color: #b7b8b9;
      }
      overflow-y: auto;
      .sidebar-nav-list {
        list-style: none;
        margin: 0;
        padding: 5px 0px 0px;
        .activeTab {
          li {
            border-left: 3px solid #1989fa;
            background-color: #e1edfa;
            .view-name {
              color: #000;
            }
          }
        }
        .sidebar-item {
          height: 45px;
          padding: 0 40px;
          margin: 0 0 5px;
          line-height: 45px;
          color: #8d9199;
          border-left: 3px solid transparent;
          border-right: 3px solid transparent;
          font-size: 14px;
          white-space: nowrap;
          position: relative;
          &:hover {
            background-color: #e1edfa;
            cursor: pointer;
          }
        }
        .sidebarCollapseItem {
          font-size: 14px;
          padding: 0 10px;
          margin: 0 0 2px;
          line-height: 45px;
          text-align: center;
        }

        .view-name {
          vertical-align: middle;
        }
        .next {
          position: absolute;
          right: 25px;
          padding: 14px 0;
        }
      }
    }
    .sidebar-collapse {
      border-top: 1px solid #e2e5ea;
      .sidebar-collapse-item {
        height: 42px;
        padding: 0 27px;
        margin: 0 0 5px;
        line-height: 45px;
        color: #8d9199;
        border-left: 3px solid transparent;
        border-right: 3px solid transparent;
        font-size: 14px;
        white-space: nowrap;
        .collapse-label {
          cursor: pointer;
          font-size: 13px;
        }
      }
    }
  }
  .sidebarCollapse {
    width: 80px;
  }
}
</style>
