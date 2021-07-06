<template>
    <div class="projects-service-mgr">
      <el-drawer title="代码源集成"
                 :visible.sync="addCodeDrawer"
                 direction="rtl">
        <add-code @cancel="addCodeDrawer = false"></add-code>
      </el-drawer>
      <el-dialog title="是否更新对应环境？"
                 custom-class="dialog-upgrade-env"
                 :visible.sync="updateEnvDialogVisible"
                 width="40%">
        <div class="title">
          <el-alert title="勾选需要更新的环境，点击确定之后，该服务将自动在对应的环境中进行更新"
                    :closable="false"
                    type="warning">
          </el-alert>
          <el-checkbox-group v-model="checkedEnvList">
            <el-checkbox v-for="(env,index) in envNameList"
                         :key="index"
                         :label="env.envName"></el-checkbox>
          </el-checkbox-group>
        </div>
        <span slot="footer"
              class="dialog-footer">
          <el-button size="small"
                     type="primary"
                     @click="autoUpgradeEnv">确 定</el-button>
          <el-button size="small"
                     @click="skipUpdate">跳过</el-button>

        </span>
      </el-dialog>
      <div class="pipeline">
        <div class="pipeline-workflow__wrap">
          <multipane class="vertical-panes"
                     layout="vertical">
            <div class="service-tree-container">
              <serviceTree :services="services"
                           :projectInfo="projectInfo"
                           :currentServiceYamlKinds="currentServiceYamlKinds"
                           :sharedServices="sharedServices"
                           :basePath="`/v1/projects/detail/${projectName}/services`"
                           :showNext.sync="showNext"
                           ref="serviceTree"
                           @onAddCodeSource="addCodeSource"
                           @onJumpToKind="jumpToKind"
                           @onRefreshService="getServices"
                           @onRefreshSharedService="getSharedServices"
                           @onSelectServiceChange="onSelectServiceChange"></serviceTree>
            </div>
            <template v-if="service.service_name  &&  services.length >0">
              <template v-if="service.type==='k8s'">
                <multipane-resizer></multipane-resizer>
                <div class="service-editor-container"
                     :style="{ minWidth: '300px', width: '500px' }"
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
                  <serviceAsideK8s :service="service"
                                   :detectedEnvs="detectedEnvs"
                                   :detectedServices="detectedServices"
                                   :systemEnvs="systemEnvs"
                                   :buildBaseUrl="`/v1/projects/detail/${projectName}/services`"
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
                       plain>环境更新</el-button>
          </div>
      </div>
    </div>
</template>
<script>
import mixin from '@utils/service_module_mixin'
import serviceAsideK8s from './k8s/service_aside.vue'
import serviceEditorK8s from './k8s/service_editor.vue'
import serviceTree from './common/service_tree.vue'
import addCode from './common/add_code.vue'
import { mapGetters } from 'vuex'
import { sortBy } from 'lodash'
import { getServiceTemplatesAPI, getServicesTemplateWithSharedAPI, saveServiceTemplateAPI, serviceTemplateWithConfigAPI, autoUpgradeEnvAPI } from '@api'
import { Multipane, MultipaneResizer } from 'vue-multipane'
export default {
  props: {
    projectInfo: Object
  },
  data () {
    return {
      service: {},
      services: [],
      sharedServices: [],
      detectedEnvs: [],
      detectedServices: [],
      systemEnvs: [],
      checkedEnvList: [],
      currentServiceYamlKinds: {},
      showNext: false,
      updateEnvDialogVisible: false,
      addCodeDrawer: false
    }
  },
  methods: {
    addCodeSource () {
      if (!this.$utils.roleCheck().superAdmin) {
        this.$message('私有镜像仓库未集成，请联系系统管理员前往「系统设置 -> 镜像仓库」进行集成！')
      } else {
        this.addCodeDrawer = true
      }
    },
    createService () {
      this.$refs.serviceTree.createService('platform')
    },
    toNext () {
      this.updateEnvDialogVisible = true
    },
    onSelectServiceChange (service) {
      this.$set(this, 'service', service)
    },
    getServices () {
      const projectName = this.projectName
      this.$set(this, 'service', {})
      getServiceTemplatesAPI(projectName).then((res) => {
        this.services = sortBy((res.data.map(service => {
          service.idStr = `${service.service_name}/${service.type}`
          service.status = 'added'
          return service
        })), 'service_name')
      })
    },
    getSharedServices () {
      const projectName = this.projectName
      getServicesTemplateWithSharedAPI(projectName).then((res) => {
        this.sharedServices = sortBy((res.map(service => {
          service.status = 'added'
          service.type = 'k8s'
          return service
        })), 'service_name')
      })
    },
    getServiceModules () {
      const serviceName = this.service.service_name
      const projectName = this.projectName
      serviceTemplateWithConfigAPI(serviceName, projectName).then(res => {
        this.detectedEnvs = res.custom_variable ? res.custom_variable : []
        this.detectedServices = res.service_module ? res.service_module : []
        this.systemEnvs = res.system_variable ? res.system_variable : []
      })
    },
    onUpdateService (payload) {
      saveServiceTemplateAPI(payload).then((res) => {
        this.showNext = true
        this.$message({
          type: 'success',
          message: '服务保存成功'
        })
        this.$router.replace({
          query: Object.assign(
            {},
            {},
            {
              service_name: payload.service_name,
              rightbar: 'var'
            })
        })
        this.getServices()
        this.$refs.serviceTree.getServiceGroup()
        this.getSharedServices()
        this.detectedEnvs = res.custom_variable ? res.custom_variable : []
        this.detectedServices = res.service_module ? res.service_module : []
        this.systemEnvs = res.system_variable ? res.system_variable : []
      })
    },
    getYamlKind (payload) {
      this.currentServiceYamlKinds = payload
    },
    jumpToKind (payload) {
      this.$refs.serviceEditor.jumpToWord(`kind: ${payload.kind}`)
    },
    async getProducts () {
      await this.$store.dispatch('getProductListSSE').closeWhenDestroy(this)
    },
    autoUpgradeEnv () {
      this.$confirm('更新环境, 是否继续?', '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        const payload = {
          env_names: this.checkedEnvList
        }
        const projectName = this.projectName
        const force = true
        autoUpgradeEnvAPI(projectName, payload, force).then((res) => {
          this.$router.push(`/v1/projects/detail/${projectName}/envs`)
          this.$message({
            message: '更新环境成功',
            type: 'success'
          })
        })
      })
    },
    updateEnv (res) {
      const message = JSON.parse(res.match(/{.+}/g)[0])
      const key = Object.keys(message)[0]
      const value = message[key].map(item => {
        return item.name + ';'
      })
      this.$confirm(`您的更新操作将覆盖集成环境中${key}的${value}服务变更，确认继续?`, '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        const payload = {
          env_names: this.checkedEnvList
        }
        const force = true
        const projectName = this.projectName
        autoUpgradeEnvAPI(projectName, payload, force).then((res) => {
          this.$router.push(`/v1/projects/detail/${projectName}/envs`)
          this.$message({
            message: '更新环境成功',
            type: 'success'
          })
        })
      })
    },
    skipUpdate () {
      this.updateEnvDialogVisible = false
    }
  },
  computed: {
    ...mapGetters([
      'productList'
    ]),

    currentOrganizationId () {
      return this.$store.state.login.userinfo.organization.id
    },
    envNameList () {
      const envNameList = []
      this.productList.forEach(element => {
        if (element.product_name === this.projectName && element.source !== 'external') {
          envNameList.push({
            envName: element.env_name
          })
        }
      })
      return envNameList
    },
    projectName () {
      return this.$route.params.project_name
    },
    serviceCount () {
      return this.services.length
    },
    serviceName () {
      return this.$route.query.service_name
    }
  },
  watch: {

  },
  mounted () {
    this.getProducts()
    this.getServices()
    this.getSharedServices()
  },
  components: {
    serviceAsideK8s,
    serviceEditorK8s,
    serviceTree,
    Multipane,
    MultipaneResizer,
    'add-code': addCode
  },
  mixins: [mixin]
}
</script>

<style lang="less">
@import "~@assets/css/component/service-mgr.less";
</style>
