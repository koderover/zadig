<template>
  <div class="projects-service-mgr">
    <el-drawer title="代码源集成"
               :visible.sync="addCodeDrawer"
               direction="rtl">
      <add-code @cancel="addCodeDrawer = false"></add-code>
    </el-drawer>
    <div class="guide-container">
      <step :activeStep="1">
      </step>
      <div class="current-step-container">
        <div class="title-container">
          <span class="first">第二步</span>
          <span class="second">创建服务模板，后续均可在项目中重新配置</span>
        </div>
      </div>
    </div>
    <div class="pipeline onboarding">
      <div class="pipeline-workflow__wrap">
        <multipane class="vertical-panes"
                   layout="vertical">
          <div class="service-tree-container">
            <serviceTree :services="services"
                         :currentServiceYamlKinds="currentServiceYamlKinds"
                         :sharedServices="sharedServices"
                         :projectInfo="projectInfo"
                         :basePath="`/v1/projects/create/${projectName}/helm/service`"
                         :guideMode="true"
                         ref="serviceTree"
                         @onAddCodeSource="addCodeDrawer = true"
                         @onJumpToKind="jumpToKind"
                         @onRefreshService="getServices"
                         @onRefreshSharedService="getSharedServices"
                         @onSelectServiceChange="onSelectServiceChange"></serviceTree>
          </div>
          <template v-if="service.service_name &&  services.length >0">
            <template v-if="service.type==='k8s'">
              <multipane-resizer></multipane-resizer>
              <div class="service-editor-container"
                   :style="{ minWidth: '300px', width: '500px'}"
                   :class="{'pm':service.type==='pm'}">
                <serviceEditorK8s ref="serviceEditor"
                                  :serviceInTree="service"
                                  :serviceCount="serviceCount"
                                  :showNext.sync="showNext"
                                  @onParseKind="getYamlKind"
                                  @onRefreshService="getServices"
                                  @onRefreshSharedService="getSharedServices"
                                  @onUpdateService="onUpdateService"></serviceEditorK8s>
              </div>
              <multipane-resizer></multipane-resizer>
              <aside class="pipelines__aside pipelines__aside_right"
                     :style="{ flexGrow: 1 }">
                <serviceAsideK8s v-if="service.product_name===projectName"
                                 :service="service"
                                 :detectedEnvs="detectedEnvs"
                                 :detectedServices="detectedServices"
                                 :systemEnvs="systemEnvs"
                                 :buildBaseUrl="`/v1/projects/create/${projectName}/basic/service`"
                                 @getServiceModules="getServiceModules"> </serviceAsideK8s>
              </aside>
            </template>
          </template>
          <div v-else
               class="no-content">
            <img src="@assets/icons/illustration/editor_nodata.svg"
                 alt="">
            <p v-if="services.length === 0">暂无服务，点击 <el-button size="mini"
                         icon="el-icon-plus"
                         @click="createService()"
                         plain
                         circle>
              </el-button> 创建服务</p>
            <p v-else-if="service.service_name==='服务列表' && services.length >0">请在左侧选择需要编辑的服务</p>
            <p v-else-if="!service.service_name && services.length >0">请在左侧选择需要编辑的服务</p>
          </div>
        </multipane>
      </div>
    </div>
    <div class="controls__wrap">
      <div class="controls__right">
        <el-button type="primary"
                   size="small"
                   class="save-btn"
                   @click="toNext"
                   :disabled="!showNext"
                   plain>下一步</el-button>
      </div>
    </div>

  </div>
</template>
<script>
import bus from '@utils/event_bus';
import mixin from '@utils/service_module_mixin';
import step from './common/step.vue';
import addCode from '../service_mgr/common/add_code.vue';
import serviceAsideK8s from './k8s/container/service_aside.vue';
import serviceEditorK8s from './k8s/container/service_editor.vue';
import serviceTree from '../service_mgr/common/service_tree.vue';
import { sortBy } from 'lodash';
import { getServiceTemplatesAPI, getServicesTemplateWithSharedAPI, saveServiceTemplateAPI, serviceTemplateWithConfigAPI, getSingleProjectAPI } from '@api';
import { Multipane, MultipaneResizer } from 'vue-multipane';
export default {
  data() {
    return {
      service: {},
      services: [],
      sharedServices: [],
      detectedEnvs: [],
      detectedServices: [],
      systemEnvs: [],
      currentServiceYamlKinds: {},
      projectInfo: {},
      showNext: false,
      addCodeDrawer: false,
    }
  },
  methods: {
    createService() {
      this.$refs['serviceTree'].createService('platform');
    },
    toNext() {
      this.$router.push(`/v1/projects/create/${this.projectName}/basic/runtime?serviceName=${this.serviceName}&serviceType=${this.serviceType}`);
    },
    onSelectServiceChange(service) {
      this.$set(this, 'service', service);
    },
    getServices() {
      const projectName = this.projectName;
      this.$set(this, 'service', {});
      getServiceTemplatesAPI(projectName).then((res) => {
        this.services = sortBy((res.data.map(service => {
          service.idStr = `${service.service_name}/${service.type}`;
          service.status = 'added';
          return service;
        })), 'service_name');
      });
    },
    getSharedServices() {
      const projectName = this.projectName;
      getServicesTemplateWithSharedAPI(projectName).then((res) => {
        this.sharedServices = sortBy((res.map(service => {
          service.status = 'added';
          service.type = 'k8s';
          return service;
        })), 'service_name');
      });
    },
    getServiceModules() {
      const serviceName = this.service.service_name;
      const projectName = this.projectName
      serviceTemplateWithConfigAPI(serviceName, projectName).then(res => {
        this.detectedEnvs = res.custom_variable ? res.custom_variable : [];
        this.detectedServices = res.service_module ? res.service_module : [];
        this.systemEnvs = res.system_variable ? res.system_variable : [];
      })
    },
    onUpdateService(payload) {
      saveServiceTemplateAPI(payload).then((res) => {
        this.showNext = true;
        this.$message({
          type: 'success',
          message: '服务保存成功'
        });
        this.$router.replace({
          query: Object.assign(
            {},
            {},
            {
              service_name: payload.service_name,
              rightbar: 'var',
            })
        });
        this.getServices();
        this.$refs.serviceTree.getServiceGroup();
        this.getSharedServices();
        this.detectedEnvs = res.custom_variable ? res.custom_variable : [];
        this.detectedServices = res.service_module ? res.service_module : [];
        this.systemEnvs = res.system_variable ? res.system_variable : [];
      })
    },
    getYamlKind(payload) {
      this.currentServiceYamlKinds = payload;
    },
    jumpToKind(payload) {
      this.$refs.serviceEditor.jumpToWord(`kind: ${payload.kind}`);
    },
    async checkProjectFeature() {
      const projectName = this.projectName;
      this.projectInfo = await getSingleProjectAPI(projectName);
    },
  },
  computed: {
    currentOrganizationId() {
      return this.$store.state.login.userinfo.organization.id;
    },
    projectName() {
      return this.$route.params.project_name;
    },
    serviceCount() {
      return this.services.length;
    },
    serviceName() {
      return this.$route.query.service_name;
    },
    serviceType() {
      return this.service.type;
    }

  },
  watch: {

  },
  mounted() {
    this.getServices();
    this.getSharedServices();
    this.checkProjectFeature();
    bus.$emit(`show-sidebar`, true);
    bus.$emit(`set-topbar-title`, { title: '', breadcrumb: [{ title: '项目', url: '/v1/projects' }, { title: this.projectName, url: '' }] });
    bus.$emit(`set-sub-sidebar-title`, {
      title: '',
      routerList: []
    });
  },
  beforeDestroy() {
    bus.$off('refresh-service');
  },
  components: {
    step, serviceAsideK8s, serviceEditorK8s, serviceTree, Multipane, MultipaneResizer, 'add-code': addCode
  },
  mixins: [mixin],
  onboardingStatus: 2
}
</script>

<style lang="less">
@import "~@assets/css/component/service-mgr.less";
</style>