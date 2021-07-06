<template>
  <div class="projects-not-k8s-config-container">
    <div class="guide-container">
      <step :activeStep="1">
      </step>
      <div class="current-step-container">
        <div class="title-container">
          <span class="first">第二步</span>
          <span class="second">配置服务的构建脚本，后续仍可在项目中重新配置</span>
        </div>
      </div>
    </div>
    <div class="config-container">
      <ServiceList ref="serviceLsit" :editService="editService" :addService="addService" :changeShowBuild="changeShowBuild"/>
      <Build  v-if="showBuild"  ref="pm-service" :serviceName="serviceName" :isEdit="isEdit"

              @listenCreateEvent="listenEvent"></Build>
      <div v-else class="no-content">
            <img src="@assets/icons/illustration/editor_nodata.svg"
                 alt="">
                <p style="color: #909399;">暂无服务，创建服务请在左侧栏点击&nbsp;<el-button size="mini"
                           icon="el-icon-plus"
                           @click="$refs.serviceLsit.newService()"
                           plain
                           circle>
                </el-button>&nbsp;创建服务</p>
      </div>
    </div>
    <div class="controls__wrap">
      <div class="controls__right">
        <button type="primary"
                size="small"
                :disabled="!showNext"
                @click="toNext"
                class="save-btn"
                plain>下一步</button>
        <div class="run-button">
        </div>
      </div>
    </div>
  </div>
</template>
<script>
import bus from '@utils/event_bus'
import step from './container/step_not_k8s.vue'
import ServiceList from '@/components/projects/common/not_k8s/service_list.vue'
import Build from '@/components/projects/common/not_k8s/not_k8s_form.vue'

export default {
  data () {
    return {
      showNext: false,
      isEdit: false,
      serviceName: '',
      showBuild: true
    }
  },
  methods: {
    addService (obj) {
      this.isEdit = false
      this.serviceName = ''
      this.$refs['pm-service'].addNewService(obj)
    },
    changeShowBuild (value) {
      this.showBuild = value
      if (value) {
        this.$nextTick(() => {
          this.$refs['pm-service'].initEnvConfig()
        })
      }
    },
    editService (obj) {
      this.serviceName = obj.service_name
      this.isEdit = true
    },
    listenEvent (res) {
      if (res === 'success') {
        this.showNext = true
        this.$refs.serviceLsit.getServiceTemplates()
      }
    },
    toNext () {
      this.$router.push(`/v1/projects/create/${this.projectName}/not_k8s/deploy`)
    }
  },
  computed: {
    projectName () {
      return this.$route.params.project_name
    },
    currentOrganizationId () {
      return this.$store.state.login.userinfo.organization.id
    }

  },
  created () {
    bus.$emit(`show-sidebar`, true)
    bus.$emit(`set-topbar-title`, { title: '', breadcrumb: [{ title: '项目', url: '/v1/projects' }, { title: this.projectName, url: '' }] })
    bus.$emit(`set-sub-sidebar-title`, {
      title: '',
      routerList: []
    })
  },
  components: {
    step, Build, ServiceList
  },
  onboardingStatus: 2
}
</script>

<style lang="less">
.no-content {
  width: 360px;
  height: 300px;
  margin: auto;

  img {
    display: block;
    width: 30%;
    height: auto;
    margin: 50px auto 0 auto;
  }
}

.highlight {
  background: #409eff57;
}

.projects-not-k8s-config-container {
  position: relative;
  flex: 1;
  height: 100%;
  overflow: auto;
  background-color: #f5f7f7;

  .page-title-container {
    display: flex;
    padding: 0 20px;

    h1 {
      width: 100%;
      color: #4c4c4c;
      font-weight: 300;
      text-align: center;
    }
  }

  .guide-container {
    margin-top: 10px;
  }

  .current-step-container {
    .title-container {
      margin-bottom: 10px;
      margin-left: 20px;

      .first {
        display: inline-block;
        width: 110px;
        padding: 8px;
        color: #fff;
        font-weight: 300;
        font-size: 18px;
        text-align: center;
        background: #3289e4;
      }

      .second {
        display: inline-block;
        color: #4c4c4c;
        font-size: 13px;
      }
    }
  }

  .config-container {
    position: relative;
    display: flex;
    height: calc(~"100% - 230px") !important;
    margin-bottom: 0;
    padding: 5px 15px 15px 15px;
  }

  .controls__wrap {
    position: relative;
    right: 0;
    bottom: 0;
    left: 0;
    z-index: 2;
    display: -webkit-box;
    display: -ms-flexbox;
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 60px;
    margin: 0 15px;
    padding: 0 10px;
    background-color: #fff;
    box-shadow: 0 4px 4px 0 rgba(0, 0, 0, 0.05);

    > * {
      margin-right: 10px;
    }

    .controls__right {
      display: -webkit-box;
      display: -ms-flexbox;
      display: flex;
      align-items: center;
      -webkit-box-align: center;
      -ms-flex-align: center;

      .save-btn,
      .next-btn {
        margin-right: 15px;
        padding: 10px 17px;
        color: #fff;
        font-weight: bold;
        font-size: 13px;
        text-decoration: none;
        background-color: #1989fa;
        border: 1px solid #1989fa;
        cursor: pointer;
        transition: background-color 300ms, color 300ms, border 300ms;
      }

      .save-btn[disabled],
      .next-btn[disabled] {
        background-color: #9ac9f9;
        border: 1px solid #9ac9f9;
        cursor: not-allowed;
      }
    }
  }
}
</style>
