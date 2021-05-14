<template>
  <div class="product-detail-container"
       ref="envContainer">
    <el-dialog title="通过工作流升级服务"
               :visible.sync="showStartProductBuild"
               custom-class="run-workflow"
               width="60%">
      <run-workflow v-if="showStartProductBuild"
                    :workflows="currentServiceWorkflows"
                    :currentServiceMeta="currentServiceMeta"
                    @success="hideProductTaskDialog"></run-workflow>
    </el-dialog>
    <div class="envs-container">
      <el-tabs v-model="envName"
               type="card">
        <el-tab-pane v-for="(env,index) in envNameList"
                     :key="index"
                     :label="`${env.envName}`"
                     :name="env.envName">
          <span slot="label">
            <i v-if="env.source==='spock'"
               class="el-icon-cloudy"></i>
            {{`${env.envName}`}}
          </span>
        </el-tab-pane>
        <el-tab-pane label="新建"
                     name="CREATE_NEW_ENV">
          <span slot="label">
            新建环境 <i class="el-icon-circle-plus-outline"></i>
          </span>
        </el-tab-pane>

      </el-tabs>
    </div>
    <!--start of basicinfo-->
    <el-card class="box-card"
             :body-style="{ padding: '0px', margin: '15px 0 0 0' }">

      <div slot="header"
           class="clearfix">
        <span>基本信息</span>
      </div>
      <div v-loading="envLoading"
           element-loading-text="正在获取环境基本信息"
           element-loading-spinner="el-icon-loading"
           class="text item">
        <el-row :gutter="10">
          <el-col :span="3">
            <div class="grid-content">更新时间:</div>
          </el-col>
          <el-col :span="8">
            <div class="grid-content">{{$utils.convertTimestamp(productInfo.update_time)}}</div>
          </el-col>
          <el-col :span="3">
            <div class="grid-content">命名空间:</div>
          </el-col>
          <el-col :span="8">
            <div class="grid-content">{{ envText }}</div>
          </el-col>
        </el-row>
        <el-row :gutter="10">
          <el-col :span="3">
            <div class="grid-content">环境状态:</div>
          </el-col>
          <el-col :span="8">
            <div class="grid-content">
              {{getEnvStatus(productInfo.status,productStatus.updatable)}}
            </div>
          </el-col>
        </el-row>
        <el-row :gutter="20">
          <el-col :span="3">
            <div class="grid-content">基本操作:</div>
          </el-col>
          <el-col :span="16">
            <div class="grid-content operation">
              <el-tooltip v-if="checkEnvUpdate(productInfo.status)"
                          content="更新环境中引用的变量"
                          effect="dark"
                          placement="top">
                <template>
                  <el-button type="text"
                             @click="openUpdateK8sVar()">更新环境变量</el-button>
                </template>
              </el-tooltip>
              <el-tooltip v-if="showUpdate(productInfo,productStatus)"
                          content="根据最新环境配置更新，包括服务编排和服务配置的改动"
                          effect="dark"
                          placement="top">
                <template v-if="productInfo.status!=='Creating'">
                  <el-button type="text"
                             @click="updateK8sEnv(productInfo)">更新环境</el-button>
                </template>
              </el-tooltip>
              <template>
                <el-button type="text"
                           @click="deleteProduct(productInfo.product_name,productInfo.env_name)">
                  删除环境</el-button>
              </template>
            </div>
          </el-col>
        </el-row>
        <el-row v-if="productInfo.error!==''"
                :gutter="20">
          <el-col :span="3">
            <div class="grid-content">错误信息:</div>
          </el-col>
          <el-col :span="16">
            <div class="grid-content error-info">
              {{productInfo.error}}
            </div>
          </el-col>
        </el-row>
      </div>
    </el-card>
    <!--end of basic info-->
    <el-card class="box-card-stack"
             :body-style="{ padding: '0px', margin: '15px 0 0 0' }">
      <div slot="header"
           class="clearfix">
        <span>服务列表</span>
        <span v-if="!serviceLoading"
              class="service-count">共计 {{ envTotal }} 个服务</span>
        <span v-if="serviceLoading"
              class="service-count"></span>
      </div>
      <div v-loading="serviceLoading"
           element-loading-text="正在获取服务信息"
           element-loading-spinner="el-icon-loading"
           class="service-container">
        <el-input size="mini"
                  class="search-input"
                  clearable
                  v-model="serviceSearch"
                  placeholder="搜索服务"
                  @keyup.enter.native="searchServicesByKeyword"
                  @clear="searchServicesByKeyword">
          <template slot="append">
            <el-button class="el-icon-search"
                       @click="searchServicesByKeyword"></el-button>
          </template>
        </el-input>
        <el-table v-if="containerServiceList.length > 0"
                  :data="containerServiceList">
          <el-table-column label="服务名"
                           width="250px">
            <template slot-scope="scope">
              <router-link :to="setRoute(scope)">
                <span :class="$utils._getStatusColor(scope.row.status)"
                      class="service-name"> <i v-if="scope.row.type==='k8s'"
                     class="iconfont service-icon iconrongqifuwu"></i>
                  {{ scope.row.service_name }}</span>
              </router-link>
              <template
                        v-if="serviceStatus[scope.row.service_name] && serviceStatus[scope.row.service_name]['tpl_updatable']">
                <el-popover placement="right"
                            popper-class="diff-popper"
                            width="600"
                            trigger="click">
                  <el-tabs v-model="activeDiffTab"
                           type="card">
                    <el-tab-pane name="template">
                      <span slot="label">
                        <i class="el-icon-tickets"></i> 模板对比
                      </span>
                      <div class="diff-container">
                        <div class="diff-content">
                          <pre :class="{ 'added': section.added, 'removed': section.removed }"
                               v-for="(section,index) in combineTemplate"
                               :key="index">{{section.value}}</pre>
                        </div>
                      </div>
                    </el-tab-pane>
                  </el-tabs>
                  <span slot="reference"
                        class="service-updateable">
                    <el-tooltip effect="dark"
                                content="配置变更"
                                placement="top">
                      <i @click="openPopper(scope.row, serviceStatus[scope.row.service_name])"
                         class="el-icon-question icon operation"></i>
                    </el-tooltip>
                  </span>
                </el-popover>
                <el-tooltip effect="dark"
                            content="更新服务"
                            placement="top">
                  <i @click="updateService(scope.row)"
                     class="iconfont icongengxin operation"></i>
                </el-tooltip>
              </template>
            </template>
          </el-table-column>
          <el-table-column align="left"
                           label="所属项目"
                           width="130px">
            <template slot-scope="scope">
              <span>{{ scope.row.product_name }}</span>
            </template>
          </el-table-column>
          <el-table-column align="left"
                           label="READY"
                           width="130px">
            <template slot-scope="scope">
              <span>{{ scope.row.ready?scope.row.ready:'N/A' }}</span>
            </template>
          </el-table-column>
          <el-table-column align="left"
                           label="状态"
                           width="130px">
            <template slot="header"
                      slot-scope="scope">状态{{`(${runningContainerService}/${containerServiceList.length})`}}
              <el-tooltip effect="dark"
                          placement="top">
                <div slot="content">实际正常的服务/预期的正常服务数量</div>
                <i class="el-icon-question"></i>
              </el-tooltip>
            </template>
            <template slot-scope="scope">
              <el-tag size="small"
                      :type="statusIndicator[scope.row.status]">
                {{scope.row.status}}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column align="left"
                           label="镜像信息">
            <template slot-scope="scope">
              <template>
                <el-tooltip v-for="(image,index) in scope.row.images"
                            :key="index"
                            effect="dark"
                            :content="image"
                            placement="top">
                  <span style="display:block">{{imageNameSplit(image) }}</span>
                </el-tooltip>
              </template>
            </template>
          </el-table-column>
          <el-table-column align="left"
                           width="150px"
                           label="服务入口">
            <template slot-scope="scope">
              <template v-if="scope.row.ingress.host_info && scope.row.ingress.host_info.length>0">
                <el-tooltip v-for="(ingress,index) in scope.row.ingress.host_info"
                            :key="index"
                            effect="dark"
                            :content="ingress.host"
                            placement="top">
                  <span class="ingress-url">
                    <a :href="`http://${ingress.host}`"
                       target="_blank">{{ingress.host}}</a>
                  </span>
                </el-tooltip>
              </template>
              <span v-else>N/A</span>
            </template>
          </el-table-column>

          <el-table-column align="center"
                           label="操作"
                           width="150px">
            <template slot-scope="scope">
              <span class="operation">
                <el-tooltip effect="dark"
                            content="通过工作流升级服务"
                            placement="top">
                  <i @click="upgradeServiceByPipe(projectName,envName,scope.row.service_name,scope.row.type)"
                     class="iconfont iconshengji"></i>
                </el-tooltip>
              </span>
              <span class="operation">
                <el-tooltip effect="dark"
                            content="重启服务"
                            placement="top">
                  <i @click="restartService(projectName,scope.row.service_name,$route.query.envName)"
                     class="el-icon-refresh"></i>
                </el-tooltip>
              </span>
              <span class="operation">
                <el-tooltip effect="dark"
                            content="查看服务配置"
                            placement="top">
                  <router-link :to="setServiceConfigRoute(scope)">
                    <i class="iconfont iconfuwupeizhi"></i>
                  </router-link>
                </el-tooltip>
              </span>
            </template>
          </el-table-column>
        </el-table>
        <p v-if="!scrollGetFlag && !serviceLoading && !scrollFinish"
           class="scroll-finish-class"><i class="el-icon-loading"></i> 数据加载中 ~</p>
        <p v-if="scrollFinish && page > 2"
           class="scroll-finish-class">数据已加载完毕 ~</p>
      </div>
    </el-card>
    <UpdateK8sVarDialog :fetchAllData="fetchAllData"
                        :productInfo="productInfo"
                        ref="updateK8sVarDialog" />
  </div>
</template>

<script>
let jsdiff = require('diff');
import { getProductStatus, serviceTypeMap } from '@utils/word_translate';
import { mapGetters } from 'vuex';
import {
  envRevisionsAPI, productEnvInfoAPI, productServicesAPI, serviceTemplateAfterRenderAPI,
  updateServiceAPI, updateK8sEnvAPI, restartServiceOriginAPI, deleteProductEnvAPI, getServicePipelineAPI, initSource, rmSource
} from '@api';
import _ from 'lodash';
import runWorkflow from './run_workflow.vue';
import UpdateK8sVarDialog from './components/update_k8s_var_dialog'

let validateKey = (rule, value, callback) => {
  if (typeof value === 'undefined' || value == '') {
    callback(new Error('请输入Key'));
  } else {
    if (!/^[a-zA-Z0-9_]+$/.test(value)) {
      callback(new Error('Key 只支持字母大小写和数字，特殊字符只支持下划线'));
    } else {
      callback();
    }
  }
};

export default {
  data() {
    return {
      ctlCancel: null,
      selectVersion: '',
      activeDiffTab: 'template',
      updataK8sEnvVarLoading: false,
      updateK8sEnvVarDialogVisible: false,
      envLoading: false,
      serviceLoading: false,
      showStartProductBuild: false,
      currentServiceWorkflows: [],
      currentServiceMeta: null,
      containerServiceList: [],
      serviceStatus: {},
      combineTemplate: [],
      productInfo: {
      },
      productStatus: {
        updateble: false
      },
      keyCheckRule: {
        key: [
          {
            type: 'string',
            required: true,
            validator: validateKey,
            trigger: 'blur'
          }
        ],
        value: [
          {
            type: 'string',
            required: false,
            message: 'value',
            trigger: 'blur'
          }
        ]
      },
      serviceTypeMap: serviceTypeMap,
      statusIndicator: {
        'Running': 'success',
        'Succeeded': 'success',
        'Error': 'danger',
        'Unstable': 'warning',
        'Unstart': 'info',
      },
      serviceSearch: '',
      page: 1,
      perPage: 20,
      envTotal: 0,
      scrollGetFlag: true,
      scrollFinish: false,
    };
  },
  computed: {
    currentOrganizationId() {
      return this.$store.state.login.userinfo.organization.id;
    },
    filteredProducts() {
      return _.uniqBy(_.orderBy(this.productList, ['product_name', 'is_prod']), 'product_name');
    },
    runningContainerService() {
      return this.containerServiceList.filter(s => (s.status === 'Running' || s.status === 'Succeeded')).length;
    },
    envText() {
      return this.productInfo.namespace;
    },
    projectName() {
      return this.$route.params.project_name;
    },
    envNameList() {
      let envNameList = [];
      let proEnvList = [];
      this.productList.forEach(element => {
        if (element.product_name === this.projectName) {
          envNameList.push({
            envName: element.env_name,
            source: element.source,
          });
        }
      });
      let res = envNameList.concat(proEnvList);
      return res;
    },
    envName: {
      get: function () {
        if (this.$route.query.envName) {
          return this.$route.query.envName;
        }
        else {
          return this.envNameList[0].envName;
        }
      },
      set: function (newValue) {
        if (newValue === 'CREATE_NEW_ENV') {
          this.$router.push({
            path: `/v1/projects/detail/${this.projectName}/envs/create`,
            query: { outer: this.envBasePath.startsWith('/v1/envs/detail') }
          });
        }
        else {
          this.$router.push({ path: `${this.envBasePath}`, query: { envName: newValue } })
        }
      }
    },
    ...mapGetters([
      'productList', 'signupStatus'
    ])
  },
  methods: {
    openUpdateK8sVar() {
      this.$refs.updateK8sVarDialog.openDialog()
    },
    searchServicesByKeyword() {
      this.initPageInfo();
      this.getProductEnv('search');
    },
    onScroll(event) {
      if (!this.scrollGetFlag) {
        return;
      }
      let target = event.target;
      let scrollTop = target.scrollTop,
        scrollHeight = target.scrollHeight,
        clientHeight = target.clientHeight;
      if (scrollTop + 1.5 * clientHeight > scrollHeight) {
        this.getProductEnv();
      }
    },
    initPageInfo() {
      this.removeListener();
      this.page = 1;
      this.envTotal = 0;
      this.scrollGetFlag = true;
      this.scrollFinish = false;
      this.containerServiceList = [];
    },
    addListener() {
      this.$refs.envContainer && this.$refs.envContainer.addEventListener('scroll', this.onScroll);
    },
    removeListener() {
      this.$refs.envContainer && this.$refs.envContainer.removeEventListener('scroll', this.onScroll);
    },
    async getProducts() {
      await this.$store.dispatch('getProductListSSE').closeWhenDestroy(this);
    },
    fetchAllData() {
      try {
        this.initPageInfo();
        this.getProductEnv();
        this.getProducts();
        this.fetchEnvRevision();
      } catch (err) {
        console.log("ERROR:" + err);
      }
    },
    fetchEnvRevision() {
      const projectName = this.projectName;
      const envName = this.envName;
      envRevisionsAPI(projectName, envName).then(revisions => {
        const productStatus = revisions.find(element => { return element.product_name === projectName && element.env_name === this.envName });
        if (productStatus.services) {
          productStatus.services.forEach(service => {
            this.$set(this.serviceStatus, service.service_name, {
              tpl_updatable: false,
              current_revision: 0,
              next_revision: 0
            });
            this.$set(this.serviceStatus, service.service_name, {
              tpl_updatable: service.updatable && service.deleted === false && service.new === false ? true : false,
              current_revision: service.current_revision,
              next_revision: service.next_revision,
              config: {
                config_name: service.configs && service.configs.length > 0 ? service.configs[0]['config_name'] : null,
                current_revision: service.configs && service.configs.length > 0 ? service.configs[0]['current_revision'] : null,
                next_revision: service.configs && service.configs.length > 0 ? service.configs[0]['next_revision'] : null,
                updatable: service.configs && service.configs.length > 0 ? service.configs[0]['updatable'] : null
              },
              raw: service
            });
          });
        }
        this.productStatus = productStatus;
      }).catch(err => {
        if (err === "CANCEL") {
          return;
        }
      });
    },
    async getProductEnv(flag) {
      const projectName = this.projectName;
      const envName = this.envName;
      try {
        let serviceGroup = [];
        if (this.page === 1 && flag !== 'search') {
          await this.getProductEnvInfo(projectName, envName);
        }
        this.scrollGetFlag = false;
        if (this.page === 1) {
          this.addListener();
        }
        const res = await productServicesAPI(projectName, envName, this.serviceSearch, this.perPage, this.page);
        this.envTotal = res.headers['x-total'] ? parseInt(res.headers['x-total']) : 0;
        serviceGroup = res.data;
        this.page++;
        this.serviceLoading = false;
        if (serviceGroup && serviceGroup.length) {
          let { containerServiceList, } = this.handleProductEnvServiceData(serviceGroup);
          this.scrollGetFlag = true;
          this.containerServiceList = this.containerServiceList.concat(containerServiceList);
          this.containerServiceList = _.orderBy(this.containerServiceList, 'service_name')
          if (this.envTotal === this.containerServiceList.length) {
            this.removeListener();
            this.scrollGetFlag = false;
            this.scrollFinish = true;
          }
        } else {
          this.removeListener();
          this.scrollGetFlag = false;
          this.scrollFinish = true;
        }
      } catch (err) {
        this.scrollGetFlag = true;
        if (err === "CANCEL") {
          return;
        }
        this.$notify.error({
          title: '获取环境信息失败'
        });
        this.$router.push(`/v1/projects/detail/${this.projectName}`);
      }
    },
    async getProductEnvInfo(projectName, envName) {
      this.envLoading = true;
      this.serviceLoading = true;
      const envInfo = await productEnvInfoAPI(projectName, envName);
      if (envInfo) {
        this.productInfo = envInfo;
        this.envLoading = false;
      }
    },
    handleProductEnvServiceData(serviceGroup) {
      let containerServiceList = this.$utils.deepSortOn(serviceGroup.filter(element => {
        return element.type === 'k8s'
      }), 'service_name');
      return {
        containerServiceList
      };
    },
    openPopper(service, service_status) {
      const product_name = this.projectName;
      const env_name = this.envName;
      serviceTemplateAfterRenderAPI(product_name, service.service_name, env_name).then((tpls) => {
        this.combineTemplate = jsdiff.diffLines(tpls.current.yaml, tpls.latest.yaml);
      });
    },
    getEnvStatus(status, updateble) {
      return getProductStatus(status, updateble);
    },
    showUpdate(product_info, product_status) {
      return product_status.updatable;
    },
    updateK8sEnv(product_info) {
      this.$confirm('更新环境, 是否继续?', '更新', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
        .then(() => {
          const projectName = product_info.product_name;
          const envName = product_info.env_name;
          const envType = '';
          const payload = { vars: product_info.vars };
          updateK8sEnvAPI(projectName, envName, payload, envType).then(
            response => {
              this.fetchAllData();
              this.$message({
                message: '更新环境成功，请等待服务升级',
                type: 'success'
              });
            }
          );
        })
        .catch(() => {
          this.$message({
            type: 'info',
            message: '已取消更新'
          });
        });
    },
    upgradeServiceByPipe(projectName, envName, serviceName, serviceType) {
      getServicePipelineAPI(projectName, envName, serviceName, serviceType).then((res) => {
        this.currentServiceWorkflows = res.workflows || [];
        this.currentServiceMeta = {
          'projectName': projectName,
          'envName': envName,
          'serviceName': serviceName,
          'serviceType': serviceType,
          'targets': res.targets || [],
          'ns': this.envText
        };
        this.showStartProductBuild = true;
      }).catch(err => {
        if (err === "CANCEL") {
          return;
        }
      });
    },
    hideProductTaskDialog() {
      this.showStartProductBuild = false;
    },
    deleteProduct(project_name, env_name) {
      const envType = '';
      this.$prompt('请输入环境名称以确认', `确定要删除 ${project_name} 项目的 ${env_name} 环境?`, {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        confirmButtonClass: 'el-button el-button--danger',
        inputValidator: input => {
          if (input == env_name) {
            return true;
          } else if (input == '') {
            return '请输入环境名称';
          } else {
            return '环境名称不相符';
          }
        }
      })
        .then(({ value }) => {
          deleteProductEnvAPI(project_name, env_name, envType).then((res) => {
            this.$notify({
              title: `环境正在删除中，请稍后查看环境状态`,
              message: '操作成功',
              type: 'success',
              offset: 50
            });
            const position = this.envNameList.map((e) => { return e.envName; }).indexOf(env_name);
            this.envNameList.splice(position, 1);
            if (this.envNameList.length > 0) {
              this.$router.push(`${this.envBasePath}?envName=${this.envNameList[this.envNameList.length - 1]['envName']}`);
            }
            else {
              this.$router.push(`/v1/projects/detail/${this.projectName}/envs/create`);
            }
          })

        })
        .catch((error) => {
          this.$message({
            type: 'warning',
            message: '取消删除'
          });
        });
    },
    restartService(projectName, serviceName, envName) {
      const envType = '';
      restartServiceOriginAPI(projectName, serviceName, envName, envType).then((res) => {
        this.$message({
          message: '重启服务成功',
          type: 'success'
        });
        this.initPageInfo();
        this.getProductEnv();
        this.fetchEnvRevision();
      })
    },
    imageNameSplit(name) {
      if (name.includes(':')) {
        return name.split('/')[name.split('/').length - 1];
      } else {
        return name;
      }
    },
    checkEnvUpdate(status) {
      if (status === 'Deleting' || status === 'Creating') {
        return false;
      }
      else {
        return true;
      }

    },
    setRoute(scope) {
      if (typeof this.envName === 'undefined') {
        return `${this.envBasePath}/${scope.row.service_name}?&projectName=${this.projectName}&namespace=${this.envText}&originProjectName=${scope.row.product_name}`;
      } else {
        return (
          `${this.envBasePath}/${scope.row.service_name}?envName=${this.envName}&&projectName=${this.projectName}&namespace=${this.envText}&originProjectName=${scope.row.product_name}`
        );
      }
    },
    setServiceConfigRoute(scope) {
      if (typeof this.envName === 'undefined') {
        return `${this.envBasePath}/${scope.row.service_name}/config?projectName=${this.projectName}&namespace=${this.envText}&originProjectName=${scope.row.product_name}`;
      } else {
        return (
          `${this.envBasePath}/${scope.row.service_name}/config?envName=${this.envName}&projectName=${this.projectName}&namespace=${this.envText}&originProjectName=${scope.row.product_name}`
        );
      }
    },
    updateService(service) {
      this.$message.info('开始更新服务');
      updateServiceAPI(
        this.projectName,
        service.service_name,
        service.type,
        this.envName,
        this.serviceStatus[service.service_name].raw
      ).then(res => {
        this.$message.success('更新成功请等待服务升级');
        this.fetchAllData();
      });
    },
  },
  created() {
    this.fetchAllData();
  },
  beforeDestroy() {
    this.removeListener();
  },
  destroyed() {
    this.ctlCancel && this.ctlCancel.cancel('CANCEL_2');
    rmSource();
  },
  watch: {
    $route(to, from) {
      if (this.projectName !== '') {
        this.ctlCancel && this.ctlCancel.cancel('CANCEL_1');
        this.ctlCancel = initSource();
        this.fetchAllData();
      }
    }
  },
  components: {
    runWorkflow,
    UpdateK8sVarDialog
  },
  props: {
    envBasePath: {
      type: String,
      required: true
    }
  }
};
</script>

<style lang="less">
@import "~@assets/css/component/env-detail.less";
</style>