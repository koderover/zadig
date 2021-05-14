<template>
  <div class="tab-menu">
    <div v-if="currentModules['basicInfo']"
         class="tab-container"
         @click="changeTab('basicInfo')">
      <div class="tab"
           :class="{'active-tab':currentTab==='basicInfo' }">
        <span class="number">基本信息</span>
      </div>
    </div>
    <div v-if="currentModules['buildDeploy']"
         class="tab-container">
      <div @click="changeTab('buildDeploy')"
           class="tab"
           :class="{'active-tab':currentTab==='buildDeploy'}">
        <span class="number">构建部署</span>
      </div>
      <span class="operation">
        <i @click="deleteModule('buildDeploy')"
           class="el-icon-error"></i>
      </span>
      <span class="operation"> </span>
    </div>
    <div v-if="currentModules['artifactDeploy']"
         class="tab-container">
      <div @click="changeTab('artifactDeploy')"
           class="tab"
           :class="{'active-tab':currentTab==='artifactDeploy'}">
        <span class="number">交付物部署</span>
      </div>
      <span class="operation">
        <i @click="deleteModule('artifactDeploy')"
           class="el-icon-error"></i>
      </span>
      <span class="operation"> </span>
    </div>
    <div v-if="currentModules['distribute']"
         class="tab-container">
      <div @click="changeTab('distribute')"
           class="tab"
           :class="{'active-tab':currentTab==='distribute' }">
        <span class="number">分发</span>
      </div>
      <span class="operation">
        <i @click="deleteModule('distribute')"
           class="el-icon-error"></i>
      </span>
      <span class="operation"> </span>
    </div>
    <!-- <div v-if="currentModules['version']"
         class="tab-container">
      <div @click="changeTab('version')"
           class="tab"
           :class="{'active-tab':currentTab==='version' }">
        <span class="number">版本</span>
      </div>
      <span class="operation">
        <i @click="deleteModule('version')"
           class="el-icon-error"></i>
      </span>
      <span class="operation"> </span>
    </div> -->
    <div v-if="currentModules['trigger']"
         class="tab-container">
      <div @click="changeTab('trigger')"
           class="tab"
           :class="{'active-tab':currentTab==='trigger' }">
        <span class="number">触发器</span>
      </div>
      <span class="operation">
        <i @click="deleteModule('trigger')"
           class="el-icon-error"></i>
      </span>
      <span class="operation"> </span>
    </div>
    <div v-if="currentModules['notify']"
         class="tab-container">
      <div @click="changeTab('notify')"
           class="tab"
           :class="{'active-tab':currentTab==='notify' }">
        <span class="number">通知</span>
      </div>
      <span class="operation">
        <i @click="deleteModule('notify')"
           class="el-icon-error"></i>
      </span>
      <span class="operation"> </span>
    </div>
  </div>
</template>

<script type="text/javascript">
import mixin from '@utils/workflow_mixin';

export default {
  data() {
    return {
      editModeOpeningTabs: {}
    };
  },
  methods: {
    changeTab(tab_name) {
      if (tab_name !== this.currentTab) {
        this.checkCurrentTab().then(() => {
          this.$emit('change-tab', tab_name);
        });
      }
    },
    deleteModule(tab_name) {
      this.$emit('delete-module', tab_name);
    },
  },
  props: {
    pipelineInfo: {
      required: true,
      type: Object
    },
    currentModules: {
      required: true,
      type: Object
    },
    currentTab: {
      required: true,
      type: String
    }
   },
  watch: {},
  created() {
  },
  mixins: [mixin]
};
</script>

<style lang="less">
.tab-menu {
  .tab-container {
    display: inline-block;
    width: 100px;
    position: relative;
    margin: 0 3px;
    .tab {
      display: inline-block;
      position: relative;
      width: 100px;
      height: 35px;
      color: rgba(255, 255, 255, 1);
      background-color: #808080ba;
      border-top-right-radius: 6px;
      border-top-left-radius: 6px;
      font-size: 14px;
      line-height: 35px;
      text-align: center;
      cursor: pointer;
      &.active-tab {
        background-color: rgba(108, 108, 108, 1);
      }
    }
    .operation {
      right: -10px;
      top: -10px;
      position: absolute;
      color: rgba(255, 87, 51, 1);
      cursor: pointer;
    }
    .info {
      display: inline-block;
      width: 100%;
      text-align: center;
      margin-top: 10px;
      .tab-name {
        display: block;
        color: rgba(0, 0, 0, 0.87);
        font-size: 13px;
      }
      .check-info {
        margin-top: 5px;
        font-size: 12px;
        display: block;
      }
      .success {
        color: rgba(103, 194, 58, 1);
      }
      .error {
        color: rgba(227, 60, 100, 1);
      }
    }
  }
  .connector {
    display: inline-block;
    height: 1px;
    width: 100px;
    background-color: rgba(153, 153, 153, 0.4);
  }
}
</style>
