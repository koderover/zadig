<template>
  <div class="projects-not-k8s-config-container">
    <UpdateEnv ref="updateEnv"/>
    <div class="config-container-pm">
      <ServiceList
        ref="serviceLsit"
        :editService="editService"
        :addService="addService"
        :changeShowBuild="(value) =>showBuild = value"
      />
      <Build
        v-if="showBuild"
        ref="pm-service"
        :serviceName="serviceName"
        :isEdit="isEdit"
        :changeUpdateEnvDisabled="changeUpdateEnvDisabled"
        @listenCreateEvent="listenEvent"
      ></Build>
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
        <button type="primary" class="save-btn" @click="openDialog" :disabled="updateEnvDisabled">
          更新环境
        </button>
      </div>
    </div>
  </div>
</template>
<script>
import ServiceList from '@/components/projects/common/not_k8s/service_list.vue'
import Build from '@/components/projects/common/not_k8s/not_k8s_form.vue'
import UpdateEnv from './not_k8s/updateEnv'
export default {
  data () {
    return {
      showNext: false,
      isEdit: false,
      serviceName: '',
      updateEnvDisabled: true,
      showBuild: true
    }
  },
  methods: {
    openDialog () {
      this.$refs.updateEnv.openDialog()
    },
    changeUpdateEnvDisabled () {
      this.updateEnvDisabled = false
    },
    addService (obj) {
      this.isEdit = false
      this.serviceName = ''
      this.$refs['pm-service'].addNewService(obj)
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
  components: {
    Build,
    ServiceList,
    UpdateEnv
  }
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

.projects-not-k8s-config-container {
  position: relative;
  flex: 1;
  height: 100%;
  overflow: auto;
  background-color: #f5f7f7;

  .config-container-pm {
    position: relative;
    display: flex;
    height: calc(~'100% - 90px') !important;
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
