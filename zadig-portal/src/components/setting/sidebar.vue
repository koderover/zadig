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
import { mapGetters } from 'vuex'
export default {
  data () {
    return {
      showSidebar: true
    }
  },
  methods: {},
  computed: {
    ...mapGetters([
      'signupStatus'
    ]),
    showClusterManage: {
      get: function () {
        if (this.signupStatus && this.signupStatus.features && this.signupStatus.features.length > 0) {
          if (this.signupStatus.features.includes('cluster_manager')) {
            return true
          } else {
            return false
          }
        }
        return false
      }
    }
  }
}
</script>

<style lang="less">
.sidebar-wrapper {
  position: relative;
  display: flex;
  height: 100%;

  .sidebar {
    display: flex;
    flex-direction: column;
    width: 230px;
    background-color: #f5f7fa;
    border-right: 1px solid #e6e9f0;
    transition: all 0.15s ease-in-out;

    .logo-wrapper {
      padding-top: 20px;
    }

    .header {
      .title {
        font-size: 15px;
      }

      text-align: center;
      vertical-align: middle;
    }

    .sidebar-nav-list-divider {
      margin: 10px 0;

      .divider {
        width: 180px;
        height: 1px;
        margin: 0 auto;
        background-color: #e6e9f0;
        transition: all 0.15s ease-in-out;
      }

      .sidebarCollapseDivider {
        width: 60px;
        transition: all 0.15s ease-in-out;
      }
    }

    .sidebar-nav-lists {
      display: flex;
      flex-direction: column;
      flex-grow: 1;
      overflow: hidden;
      overflow-y: auto;

      &::-webkit-scrollbar-track {
        background-color: #f5f5f5;
        border-radius: 6px;
      }

      &::-webkit-scrollbar {
        width: 6px;
        background-color: #f5f5f5;
      }

      &::-webkit-scrollbar-thumb {
        background-color: #b7b8b9;
        border-radius: 6px;
      }

      .sidebar-nav-list {
        margin: 0;
        padding: 5px 0 0;
        list-style: none;

        .view-name {
          vertical-align: middle;
        }

        .activeTab {
          li {
            background-color: #e1edfa;
            border-left: 3px solid #1989fa;

            .view-name {
              color: #000;
            }
          }
        }

        .sidebar-item {
          position: relative;
          height: 45px;
          margin: 0 0 5px;
          padding: 0 40px;
          color: #8d9199;
          font-size: 14px;
          line-height: 45px;
          white-space: nowrap;
          border-right: 3px solid transparent;
          border-left: 3px solid transparent;

          &:hover {
            background-color: #e1edfa;
            cursor: pointer;
          }
        }

        .sidebarCollapseItem {
          margin: 0 0 2px;
          padding: 0 10px;
          font-size: 14px;
          line-height: 45px;
          text-align: center;
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
        margin: 0 0 5px;
        padding: 0 27px;
        color: #8d9199;
        font-size: 14px;
        line-height: 45px;
        white-space: nowrap;
        border-right: 3px solid transparent;
        border-left: 3px solid transparent;

        .collapse-label {
          font-size: 13px;
          cursor: pointer;
        }
      }
    }
  }

  .sidebarCollapse {
    width: 80px;
  }
}
</style>
