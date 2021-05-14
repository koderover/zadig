<template>
  <div class="product-pipeline-detail">
    <sideMenu @add-module="addModule($event)"
              :pipelineInfo="pipelineInfo"
              :currentModules="currentModules"
              :currentTab="currentTab"></sideMenu>
    <div class="tab-page">
      <div class="tab-menu-container">
        <tabMenu @delete-module="deleteModule($event)"
                 @change-tab="changeTab($event)"
                 :pipelineInfo="pipelineInfo"
                 :currentModules="currentModules"
                 :currentTab="currentTab"
                 class="tabMenu"></tabMenu>
      </div>
      <basicInfo v-show="currentTab==='basicInfo'"
                 :editMode="editMode"
                 :pipelineInfo="pipelineInfo"></basicInfo>
      <buildDeploy v-show="currentTab==='buildDeploy'"
                   :editMode="editMode"
                   :build_stage="pipelineInfo.build_stage"
                   :product_tmpl_name="pipelineInfo.product_tmpl_name"
                   :presets="presets"></buildDeploy>
      <artifactDeploy v-show="currentTab==='artifactDeploy'"
                      :editMode="editMode"
                      :artifact_stage="pipelineInfo.artifact_stage"
                      :product_tmpl_name="pipelineInfo.product_tmpl_name"
                      :presets="presets"></artifactDeploy>
      <distribute v-show="currentTab==='distribute'"
                  :editMode="editMode"
                  :distribute_stage="pipelineInfo.distribute_stage"
                  :product_tmpl_name="pipelineInfo.product_tmpl_name"
                  :presets="presets"
                  :buildTargets="buildTargets"></distribute>
      <notify v-show="currentTab==='notify'"
              :editMode="editMode"
              :notify="pipelineInfo.notify_ctl"></notify>
      <trigger v-show="currentTab==='trigger'"
               :editMode="editMode"
               :productTmlName="pipelineInfo.product_tmpl_name"
               :presets="presets"
               :workflowToRun="pipelineInfo"
               :schedules="pipelineInfo.schedules"
               :webhook="pipelineInfo.hook_ctl"></trigger>
    </div>

    <footer class="create-footer">
      <el-row>
        <el-col :span="12">
          <div class="grid-content bg-purple">
            <div class="description">
              <el-tag type="primary">填写相关信息，然后点击保存</el-tag>
            </div>
          </div>
        </el-col>
        <el-col :span="12">
          <div class="grid-content button-container">
            <el-button @click="stepBack"
                       type="primary"
                       class="btn-primary"
                       style="margin-right: 15px;">取消</el-button>
            <el-button class="btn-primary"
                       @click="savePipeline()"
                       type="primary">保存</el-button>
          </div>
        </el-col>
      </el-row>
    </footer>
  </div>

</template>

<script>
import mixin from '@utils/workflow_mixin';
import sideMenu from './side_menu.vue';
import tabMenu from './tab_menu.vue';
import basicInfo from './switch_tab/basic_info.vue';
import buildDeploy from './switch_tab/build_deploy.vue';
import artifactDeploy from './switch_tab/artifact_deploy.vue';
import distribute from './switch_tab/distribute.vue';
import trigger from './switch_tab/trigger.vue';
import notify from './switch_tab/notify.vue';

import { workflowAPI, workflowPresetAPI, createWorkflowAPI, updateWorkflowAPI } from '@api';

export default {
  data() {
    return {
      fromRoute: null,
      pipelineInfo: {
        name: '',
        enabled: true,
        product_tmpl_name: '',
        team: '',
        description: '',
        notify_ctl: {
          enabled: false,
          weChat_webHook: '',
          notify_type: [],
        },
        build_stage: {
          enabled: false,
          modules: []
        },
        artifact_stage: {
          enabled: false,
          modules: []
        },
        distribute_stage: {
          enabled: false,
          image_repo: '',
          s3_storage_id: '',
          distributes: []
        },
        hook_ctl: {
          enabled: false,
          items: []
        },
        schedule_enabled: false,
        schedules: {
          enabled: false,
          items: [
          ]
        }
      },

      presets: [],

      currentTab: 'basicInfo',
      currentModules: {
        basicInfo: true,
        buildDeploy: false,
        artifactDeploy: false,
        distribute: false,
        notify: false,
      }
    };
  },
  beforeRouteEnter(to, from, next) {
    next(vm => {
      vm.fromRoute = from;
    })
  },
  computed: {
    pipelineName() {
      return this.$route.params.name;
    },
    editMode() {
      return Boolean(this.pipelineName);
    },
    buildTargets() {
      return this.pipelineInfo.build_stage.modules.map(m => m.target);
    },
  },
  watch: {
    'pipelineInfo.product_tmpl_name'(val) {
      workflowPresetAPI(val).then(res => {
        this.presets = res;
      });
    },
  },
  methods: {
    addModule(module_name) {
      if (module_name === 'buildDeploy') {
        this.currentModules[module_name] = true;
        this.setModuleEnabled(module_name, true);
        this.currentModules['artifactDeploy'] = false
        this.setModuleEnabled('artifactDeploy', false);
      }
      else if (module_name === 'artifactDeploy') {
        this.currentModules[module_name] = true;
        this.setModuleEnabled(module_name, true);
        this.currentModules['buildDeploy'] = false
        this.setModuleEnabled('buildDeploy', false);
      }

      else if (!this.currentModules[module_name]) {
        this.currentModules[module_name] = true;
        this.setModuleEnabled(module_name, true);
      }

      this.currentTab = module_name;
    },
    deleteModule(module_name) {
      this.currentModules[module_name] = false;
      this.setModuleEnabled(module_name, false);
      if (this.currentTab === module_name) {
        this.currentTab = this.changeToTheLastModule();
      }
    },
    changeToTheLastModule() {
      let moduleArray = [];
      for (const key in this.currentModules) {
        if (this.currentModules.hasOwnProperty(key) && this.currentModules[key]) {
          moduleArray.push(key);
        }
      }
      return moduleArray[moduleArray.length - 1];
    },
    changeTab(tab) {
      this.currentTab = tab;
    },
    setModuleEnabled(alias, isEnabled) {
      if (alias === 'buildDeploy') {
        this.pipelineInfo.build_stage.enabled = isEnabled;
        return;
      }
      if (alias === 'artifactDeploy') {
        this.pipelineInfo.artifact_stage.enabled = isEnabled;
        return;
      }

      if (alias === 'distribute') {
        this.pipelineInfo.distribute_stage.enabled = isEnabled;
        return;
      }

      if (alias === 'trigger') {
        this.pipelineInfo.hook_ctl.enabled = false;
        this.pipelineInfo.schedules.enabled = false;
        return;
      }

      if (alias === 'notify') {
        this.pipelineInfo.notify_ctl.enabled = isEnabled;
        return;
      }
    },

    savePipeline() {
      this.pipelineInfo.schedule_enabled = this.pipelineInfo.schedules.enabled;
      this.checkCurrentTab().then(() => {
        if (this.pipelineInfo.distribute_stage.enabled) {
          if (this.pipelineInfo.distribute_stage.releaseIds && this.pipelineInfo.distribute_stage.releaseIds.length > 0) {
            const releases = this.pipelineInfo.distribute_stage.releaseIds.map(element => {
              return { 'repo_id': element };
            });
            this.$set(this.pipelineInfo.distribute_stage, 'releases', releases);
          }
          else {
            this.$set(this.pipelineInfo.distribute_stage, 'releases', []);
          }
        }
        (this.editMode ? updateWorkflowAPI : createWorkflowAPI)(this.pipelineInfo).then(res => {
          this.$message.success('保存成功');
          this.$store.dispatch('refreshWorkflowList');
          if (this.$route.query.from) {
            this.$router.push(this.$route.query.from);
          }
          else {
            this.$router.push(`/v1/projects/detail/${this.pipelineInfo.product_tmpl_name}/pipelines/multi/${this.pipelineInfo.name}`);
          }
        });
      });
    },

    stepBack() {
      const name = this.pipelineName;
      if (this.$route.query.from) {
        this.$router.push(this.$route.query.from);
      }
      else if (this.fromRoute.path) {
        this.$router.push(this.fromRoute.path);
      }
      else if (name) {
        this.$router.push(`/v1/projects/detail/${this.projectName}/pipelines/multi/${name}`);
      }
      else if (window.history.length > 1) {
        this.$router.back();
      }
      else {
        this.$router.push('/v1/pipelines');
      }
    }
  },
  created() {
    if (this.editMode) {
      workflowAPI(this.pipelineName).then(res => {
        this.pipelineInfo = res;
        if (!this.pipelineInfo.schedules) {
          this.$set(this.pipelineInfo, 'schedules', {
            enabled: false,
            items: []
          })
        };
        if (!this.pipelineInfo.hook_ctl) {
          this.$set(this.pipelineInfo, 'hook_ctl', {
            enabled: false,
            items: []
          })
        };
        if (!this.pipelineInfo.notify_ctl) {
          this.$set(this.pipelineInfo, 'notify_ctl', {
            enabled: false,
            weChat_webHook: '',
            notify_type: [],
          })
        };
        if (!this.pipelineInfo.artifact_stage) {
          this.$set(this.pipelineInfo, 'artifact_stage', {
            enabled: false,
            modules: []
          })
        };

        if (res.distribute_stage.enabled) {
          if (this.pipelineInfo.distribute_stage.releases && this.pipelineInfo.distribute_stage.releases.length > 0) {
            const releaseIds = this.pipelineInfo.distribute_stage.releases.map(element => {
              return element.repo_id;
            });
            this.$set(this.pipelineInfo.distribute_stage, 'releaseIds', releaseIds);
          }
          else {
            this.$set(this.pipelineInfo.distribute_stage, 'releaseIds', []);
          }
        }
        this.currentModules = {
          basicInfo: true,
          buildDeploy: res.build_stage.enabled,
          artifactDeploy: res.artifact_stage.enabled || (res.artifact_stage && res.artifact_stage.enabled),
          distribute: res.distribute_stage.enabled,
          notify: res.notify_ctl.enabled,
          trigger: res.hook_ctl.enabled || res.schedules.enabled
        };
      });
    }
  },
  components: {
    sideMenu,
    tabMenu,
    basicInfo,
    buildDeploy,
    artifactDeploy,
    distribute,
    trigger,
    notify,
  },
  mixins: [mixin]
};
</script>

<style lang="less">
.product-pipeline-detail {
  display: flex;
  width: 100%;
  .tab-menu-container {
    margin-top: 20px;
  }
  .tab-page {
    display: block;
    position: absolute;
    bottom: 0;
    left: 50%;
    width: 80%;
    top: 5%;
    margin-left: -40%;
  }
  .create-footer {
    position: fixed;
    box-sizing: border-box;
    right: 130px;
    width: 400px;
    border-radius: 4px;
    bottom: 0;
    padding: 10px 10px 10px 10px;
    z-index: 5;
    text-align: left;
    background-color: transparent;
    .btn-primary {
      color: #1989fa;
      background-color: rgba(25, 137, 250, 0.04);
      border-color: rgba(25, 137, 250, 0.4);
      &:hover {
        color: #fff;
        background-color: #1989fa;
        border-color: #1989fa;
      }
    }
    .grid-content {
      border-radius: 4px;
      min-height: 36px;
      .description {
        line-height: 36px;
        p {
          margin: 0;
          text-align: left;
          line-height: 36px;
          font-size: 16px;
          color: #676767;
        }
      }
      &.button-container {
        float: right;
      }
    }
  }
}
</style>
