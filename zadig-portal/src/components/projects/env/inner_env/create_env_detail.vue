<template>
    <div class="create-product-detail-container"
         v-loading="loading"
         element-loading-text="正在加载中"
         element-loading-spinner="el-icon-loading">
      <el-drawer :wrapperClosable="false"
                 title="变量编辑"
                 :visible.sync="showHelmVarEdit"
                 direction="rtl"
                 size="40%"
                 custom-class="helm-yaml-drawer"
                 ref="drawer">
        <div class="helm-yaml-drawer__content">
          <editor v-model="currentEditHelmYaml.value"
                  :options="yamlEditorOption"
                  lang="yaml"
                  theme="xcode"
                  width="100%"
                  height="500"></editor>
          <div class="helm-yaml-drawer__footer">
            <el-button @click="$refs.drawer.closeDrawer()">取 消</el-button>
            <el-button @click="saveHelmValue(currentEditHelmYaml.index,currentEditHelmYaml.value)"
                       type="primary">确定</el-button>
          </div>
        </div>
      </el-drawer>

      <div class="module-title">
        <h1>创建环境</h1>
      </div>
      <div v-if="showEmptyServiceModal"
           class="no-resources">
        <div>
          <img src="@assets/icons/illustration/environment.svg"
               alt="">
        </div>
        <div class="description">
          <p>1.该环境暂无服务，请点击
            <router-link :to="`/v1/projects/detail/${projectName}/services`">
              <el-button type="primary"
                         size="mini"
                         round
                         plain>项目->服务</el-button>
            </router-link>
            添加服务
          </p>
          <p>2.如需托管外部环境，请点击
            <el-button type="primary"
                       size="mini"
                       round
                       @click="changeCreateMethodWhenServiceEmpty"
                       plain>托管环境</el-button>
            开始托管
          </p>
        </div>
      </div>
      <div v-else>
        <el-form label-width="200px"
                 ref="create-env-ref"
                 :model="projectConfig"
                 :rules="rules">
          <el-form-item label="环境名称："
                        prop="env_name">
            <el-input v-model="projectConfig.env_name"
                      size="small"></el-input>
          </el-form-item>
          <el-form-item label="创建方式"
                        prop="source">
            <el-select class="select"
                       @change="changeCreateMethod"
                       v-model="projectConfig.source"
                       size="small"
                       placeholder="请选择环境类型">
              <el-option label="系统创建"
                         value="system">
              </el-option>
              <el-option label="托管外部环境"
                         value="external">
              </el-option>
              <el-option v-if="currentProductDeliveryVersions.length > 0" label="版本回溯"
                         value="versionBack">
              </el-option>
            </el-select>
          </el-form-item>
          <el-form-item v-if="projectConfig.source==='versionBack'"
                        label="选择版本"
                        >
            <el-select @change="changeSelectValue"
                      placeholder="请选择版本"
                      size="small"
                      v-model="selection"
                      value-key="version"
                      >
              <el-option v-for="(item,index) in currentProductDeliveryVersions"
                        :key="index"
                        :disabled="!item.versionInfo.productEnvInfo"
                        :label="`版本号：${item.versionInfo.version} 创建时间：${$utils.convertTimestamp(item.versionInfo.created_at)} 创建人：${item.versionInfo.createdBy}`"
                        :value="item.versionInfo">
              </el-option>
            </el-select>
          </el-form-item>
          <el-form-item label="集群："
                        prop="cluster_id">
            <el-select class="select"
                       filterable
                       @change="changeCluster"
                       v-model="projectConfig.cluster_id"
                       size="small"
                       placeholder="请选择集群">
              <el-option label="本地集群"
                         value="">
              </el-option>
              <el-option v-for="cluster in allCluster"
                         :key="cluster.id"
                         :label="`${cluster.name} （${cluster.production?'生产集群':'测试集群'})`"
                         :value="cluster.id">
              </el-option>
            </el-select>
          </el-form-item>
          <el-form-item v-if="projectConfig.source==='external'"
                        label="命名空间"
                        prop="namespace">
            <el-select class="select"
                       v-model.trim="projectConfig.namespace"
                       size="small"
                       placeholder="请选择命名空间"
                       allow-create
                       filterable>
              <el-option v-for="(ns,index) in hostingNamespace"
                         :key="index"
                         :label="ns.name"
                         :value="ns.name">
              </el-option>
            </el-select>
          </el-form-item>
        </el-form>

        <el-card v-if="(deployType===''||deployType==='k8s') && projectConfig.vars && projectConfig.vars.length > 0  && !$utils.isEmpty(containerMap) && projectConfig.source==='system'"
                 class="box-card-service"
                 :body-style="{padding: '0px', margin: '10px 0 0 0'}">
          <div class="module-title">
            <h1>变量列表</h1>
          </div>
          <div class="kv-container">
            <el-table :data="projectConfig.vars"
                      style="width: 100%;">
              <el-table-column label="Key">
                <template slot-scope="scope">
                  <span>{{ scope.row.key }}</span>
                </template>
              </el-table-column>
              <el-table-column label="Value">
                <template slot-scope="scope">
                  <el-input size="small"
                            v-model="scope.row.value"
                            type="textarea"
                            :disabled="rollbackMode"
                            :autosize="{ minRows: 1, maxRows: 4}"
                            placeholder="请输入内容"></el-input>
                </template>
              </el-table-column>
              <el-table-column label="关联服务">
                <template slot-scope="scope">
                  <span>{{ scope.row.services?scope.row.services.join(','):'-' }}</span>
                </template>
              </el-table-column>
              <el-table-column label="操作"
                               width="150">
                <template slot-scope="scope">
                  <template>
                    <span class="operate">
                      <el-button v-if="scope.row.state === 'unused'"
                                 type="text"
                                 @click="deleteRenderKey(scope.$index,scope.row.state)"
                                 class="delete">移除</el-button>
                      <el-tooltip v-else
                                  effect="dark"
                                  content="模板中用到的渲染 Key 无法被删除"
                                  placement="top">
                        <span class="el-icon-question"></span>
                      </el-tooltip>
                    </span>
                  </template>
                </template>
              </el-table-column>
            </el-table>
            <div v-if="addKeyInputVisable"
                 class="add-key-container">
              <el-table :data="addKeyData"
                        :show-header="false"
                        style="width: 100%;">
                <el-table-column>
                  <template slot-scope="{ row }">
                    <el-form :model="row"
                             :rules="keyCheckRule"
                             ref="addKeyForm"
                             hide-required-asterisk>
                      <el-form-item label="Key"
                                    prop="key"
                                    inline-message>
                        <el-input size="small"
                                  type="textarea"
                                  :autosize="{ minRows: 1, maxRows: 4}"
                                  v-model="row.key"
                                  placeholder="请输入 Key">
                        </el-input>
                      </el-form-item>
                    </el-form>
                  </template>
                </el-table-column>
                <el-table-column>
                  <template slot-scope="{ row }">
                    <el-form :model="row"
                             :rules="keyCheckRule"
                             ref="addValueForm"
                             hide-required-asterisk>
                      <el-form-item label="Value"
                                    prop="value"
                                    inline-message>
                        <el-input size="small"
                                  type="textarea"
                                  :autosize="{ minRows: 1, maxRows: 4}"
                                  v-model="row.value"
                                  placeholder="请输入 Value">
                        </el-input>
                      </el-form-item>
                    </el-form>
                  </template>
                </el-table-column>
                <el-table-column width="100">
                  <template>
                    <span style="display: inline-block; margin-bottom: 15px;">
                      <el-button @click="addRenderKey()"
                                 type="text">确认</el-button>
                      <el-button @click="addKeyInputVisable=false"
                                 type="text">取消</el-button>
                    </span>
                  </template>
                </el-table-column>
              </el-table>
            </div>
            <span v-if="!rollbackMode"
                  class="add-kv-btn">
              <i title="添加"
                 @click="addKeyInputVisable=true"
                 class="el-icon-circle-plus"> 新增</i>
            </span>
          </div>
        </el-card>
        <div v-if="projectConfig.source==='system'">
          <div style="color: rgb(153, 153, 153); font-size: 16px; line-height: 20px;">服务列表</div>
          <template v-if="deployType==='k8s'">
            <el-card v-if="!$utils.isEmpty(containerMap)"
                    class="box-card-service"
                    :body-style="{padding: '0px'}">
              <div slot="header"
                  class="clearfix">
                <span class="second-title">
                  微服务 K8s YAML 部署
                </span>
                <span class="service-filter">
                  快速过滤:
                  <el-tooltip class="img-tooltip"
                              effect="dark"
                              placement="top">
                    <div slot="content">智能选择会优先选择最新的容器镜像，如果在 Registry<br />
                      下不存在该容器镜像，则会选择模板中的默认镜像进行填充</div>
                    <i class="el-icon-info"></i>
                  </el-tooltip>
                  <el-select :disabled="rollbackMode"
                            size="small"
                            class="img-select"
                            v-model="quickSelection"
                            placeholder="请选择">
                    <el-option label="全容器-智能选择镜像"
                              value="latest"></el-option>
                    <el-option label="全容器-全部默认镜像"
                              value="default"></el-option>
                  </el-select>
                </span>
              </div>

              <el-form class="service-form"
                      label-width="190px">
                <div class="group"
                    v-for="(typeServiceMap, serviceName) in containerMap"
                    :key="serviceName">
                  <el-tag>{{ serviceName }}</el-tag>
                  <div class="service">
                    <div v-for="service in typeServiceMap"
                        :key="`${service.service_name}-${service.type}`"
                        class="service-block">

                      <div v-if="service.type==='k8s' && service.containers"
                          class="container-images">
                        <el-form-item v-for="con of service.containers"
                                      :key="con.name"
                                      :label="con.name">
                          <el-select v-model="con.image"
                                    :disabled="rollbackMode"
                                    filterable
                                    size="small">
                            <el-option v-for="img of imageMap[con.name]"
                                      :key="`${img.name}-${img.tag}`"
                                      :label="img.tag"
                                      :value="img.full"></el-option>
                          </el-select>
                        </el-form-item>
                      </div>

                    </div>
                  </div>
                </div>
              </el-form>
            </el-card>
            <el-card v-if="!$utils.isEmpty(pmServiceMap)"
                    class="box-card-service"
                    :body-style="{padding: '0px'}">
              <div slot="header"
                  class="clearfix">
                <span class="second-title">
                  单服务或微服务(自定义脚本/Docker 部署)
                </span>
                <span class="small-title">
                  (请关联服务的主机资源，后续也可以在服务中进行配置)
                </span>
              </div>

              <el-form class="service-form"
                      label-width="190px">
                <div class="group"
                    v-for="(typeServiceMap, serviceName) in pmServiceMap"
                    :key="serviceName">
                  <el-tag>{{ serviceName }}</el-tag>
                  <div class="service">
                    <div v-for="service in typeServiceMap"
                        :key="`${service.service_name}-${service.type}`"
                        class="service-block">
                      <div v-if="service.type==='pm'"
                          class="container-images">
                        <el-form-item label="请关联主机资源：">
                          <el-button v-if="allHost.length===0"
                                    @click="createHost"
                                    type="text">创建主机</el-button>
                          <el-select v-else
                                    v-model="service.host_ids"
                                    :disabled="rollbackMode"
                                    filterable
                                    multiple
                                    placeholder="请选择要关联的主机"
                                    size="small">
                            <el-option v-for="(host,index) in  allHost"
                                      :key="index"
                                      :label="`${host.name}-${host.ip}`"
                                      :value="host.id">
                            </el-option>
                          </el-select>
                        </el-form-item>
                      </div>
                    </div>
                  </div>
                </div>
              </el-form>
            </el-card>
          </template>
          <template v-if="deployType==='helm'">
            <el-card v-if="!$utils.isEmpty(helmServiceMap)"
                    class="box-card-service"
                    :body-style="{padding: '0px'}">
              <div slot="header"
                  class="clearfix">
                <span class="second-title">
                  Chart (HELM 部署)
                </span>
                <span class="small-title">

                </span>
              </div>
              <el-table :data="helmCharts"
                        style="width: 100%;">
                <el-table-column prop="service_name"
                                label="名称">
                </el-table-column>
                <el-table-column prop="chart_version"
                                label="版本">
                  <template slot-scope="scope">
                    <el-select v-model="scope.row.chart_version"
                              size="small"
                              disabled
                              placeholder="请选择">
                      <el-option v-for="(item,index) in chartVersionMap[scope.row.service_name]"
                                :key="index"
                                :label="item.version"
                                :value="item.version">
                      </el-option>
                    </el-select>
                  </template>
                </el-table-column>
                <el-table-column label="操作">
                  <template slot-scope="scope">
                    <el-button type="primary"
                              size="mini"
                              @click="editHelmValue(scope.$index, scope.row)">变量修改</el-button>
                  </template>
                </el-table-column>
              </el-table>
            </el-card>
          </template>
        </div>
        <el-form label-width="200px"
                 class="ops">
          <el-form-item>
            <el-button @click="startDeploy"
                       :loading="startDeployLoading"
                       type="primary"
                       size="medium">确定</el-button>
            <el-button @click="goBack"
                       :loading="startDeployLoading"
                       size="medium">取消</el-button>
          </el-form-item>
        </el-form>
        <footer v-if="startDeployLoading"
                class="create-footer">
          <el-row :gutter="20">
            <el-col :span="16">
              <div class="grid-content bg-purple">
                <div class="description">
                  <el-tag type="primary">正在创建环境中....</el-tag>
                </div>
              </div>
            </el-col>

            <el-col :span="8">
              <div class="grid-content bg-purple">
                <div class="deploy-loading">
                  <div class="spinner__item1"></div>
                  <div class="spinner__item2"></div>
                  <div class="spinner__item3"></div>
                  <div class="spinner__item4"></div>
                </div>
              </div>
            </el-col>
          </el-row>
        </footer>
      </div>
    </div>
</template>

<script>
import {
  imagesAPI, productHostingNamespaceAPI, initProductAPI, getVersionListAPI, getClusterListAPI, createProductAPI, getSingleProjectAPI, getHostListAPI
} from '@api'
import bus from '@utils/event_bus'
import { mapGetters } from 'vuex'
import { uniq, cloneDeep } from 'lodash'
import { serviceTypeMap } from '@utils/word_translate'
import aceEditor from 'vue2-ace-bind'
import 'brace/mode/yaml'
import 'brace/theme/xcode'
import 'brace/ext/searchbox'

const validateKey = (rule, value, callback) => {
  if (typeof value === 'undefined' || value === '') {
    callback(new Error('请输入Key'))
  } else {
    if (!/^[a-zA-Z0-9_]+$/.test(value)) {
      callback(new Error('Key 只支持字母大小写和数字，特殊字符只支持下划线'))
    } else {
      callback()
    }
  }
}
const validateEnvName = (rule, value, callback) => {
  if (typeof value === 'undefined' || value === '') {
    callback(new Error('填写环境名称'))
  } else {
    if (!/^[a-z0-9-]+$/.test(value)) {
      callback(new Error('环境名称只支持小写字母和数字，特殊字符只支持中划线'))
    } else {
      callback()
    }
  }
}
export default {
  data () {
    return {
      selection: '',
      currentProductDeliveryVersions: [],
      projectConfig: {
        product_name: '',
        cluster_id: '',
        env_name: '',
        source: 'spock',
        vars: [],
        revision: null,
        isPublic: true,
        roleIds: []
      },
      projectInfo: {},
      yamlEditorOption: {
        enableEmmet: true,
        showLineNumbers: false,
        showGutter: false,
        showPrintMargin: false,
        tabSize: 2
      },
      hostingNamespace: [],
      allHost: [],
      roles: [],
      allCluster: [],
      startDeployLoading: false,
      loading: false,
      addKeyInputVisable: false,
      showHelmVarEdit: false,
      imageMap: {},
      containerMap: {},
      pmServiceMap: {},
      helmServiceMap: {},
      chartVersionMap: {},
      helmCharts: [],
      quickSelection: '',
      currentEditHelmYaml: {
        index: null,
        value: ''
      },
      unSelectedImgContainers: [],
      serviceTypeMap: serviceTypeMap,
      rules: {
        cluster_id: [
          { required: false, trigger: 'change', message: '请选择集群' }
        ],
        source: [
          { required: true, trigger: 'change', message: '请选择环境类型' }
        ],
        namespace: [
          { required: true, trigger: 'change', message: '请选择命名空间' }
        ],
        env_name: [
          { required: true, trigger: 'change', validator: validateEnvName }
        ],
        roleIds: [
          { type: 'array', required: true, message: '请选择项目角色', trigger: 'change' }
        ]
      },
      addKeyData: [
        {
          key: '',
          value: '',
          state: 'unused'
        }
      ],
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
      }
    }
  },

  computed: {
    ...mapGetters(['signupStatus']),
    projectName () {
      return this.$route.params.project_name
    },
    deployType () {
      return this.projectInfo.product_feature ? this.projectInfo.product_feature.deploy_type : 'k8s'
    },
    currentOrganizationId () {
      return this.$store.state.login.userinfo.organization.id
    },
    rollbackId () {
      return this.$route.query.rollbackId
    },
    rollbackMode () {
      return this.projectConfig.source === 'versionBack'
    },
    showEmptyServiceModal () {
      return this.$utils.isEmpty(this.containerMap) && this.$utils.isEmpty(this.pmServiceMap) && this.$utils.isEmpty(this.helmServiceMap) && (this.projectConfig.source !== 'external')
    }
  },
  methods: {
    async getCluster () {
      const res = await getClusterListAPI()
      if (!this.rollbackMode) {
        this.allCluster = res.filter(element => {
          return element.status === 'normal'
        })
      } else if (this.rollbackMode) {
        this.allCluster = res.filter(element => {
          return (element.status === 'normal' && !element.production)
        })
      }
    },
    async checkProjectFeature () {
      const projectName = this.projectName
      this.projectInfo = await getSingleProjectAPI(projectName)
    },
    changeSelectValue (versionInfo) {
      const template = versionInfo.productEnvInfo
      const source = this.projectConfig.source
      const env_name = this.projectConfig.env_name
      this.projectConfig = cloneDeep(template)

      for (const group of template.services) {
        group.sort((a, b) => {
          if (a.service_name !== b.service_name) {
            return a.service_name.charCodeAt(0) - b.service_name.charCodeAt(0)
          }
          if (a.type === 'k8s' || b.type === 'k8s') {
            return a.type === 'k8s' ? 1 : -1
          }
          return 0
        })
      }
      const map = {}
      for (const group of template.services) {
        for (const ser of group) {
          map[ser.service_name] = map[ser.service_name] || {}
          map[ser.service_name][ser.type] = ser
          if (ser.type === 'k8s') {
            this.hasK8s = true
          }
          ser.picked = true
          const containers = ser.containers
          if (containers) {
            for (const con of containers) {
              Object.defineProperty(con, 'defaultImage', {
                value: con.image,
                enumerable: false,
                writable: false
              })
            }
          }
        }
      }
      if (template.source === '' || template.source === 'spock' || template.source === 'helm') {
        this.projectConfig.source = 'system'
      }
      if (source === 'versionBack') {
        this.projectConfig.source = 'versionBack'
      }
      this.projectConfig.env_name = env_name
      this.projectConfig.cluster_id = ''
      this.containerMap = map
    },
    getVersionList () {
      const orgId = this.currentOrganizationId
      const projectName = this.projectName
      getVersionListAPI(orgId, '', projectName).then((res) => {
        this.currentProductDeliveryVersions = res
      })
    },
    async getTemplateAndImg () {
      this.loading = true
      const template = await initProductAPI(this.projectName, this.isStcov)
      this.loading = false
      this.projectConfig.revision = template.revision
      this.projectConfig.vars = template.vars
      this.helmCharts = template.chart_infos
      if (template.source === '' || template.source === 'spock' || template.source === 'helm') {
        this.projectConfig.source = 'system'
      };
      for (const group of template.services) {
        group.sort((a, b) => {
          if (a.service_name !== b.service_name) {
            return a.service_name.charCodeAt(0) - b.service_name.charCodeAt(0)
          }
          if (a.type === 'k8s' || b.type === 'k8s') {
            return a.type === 'k8s' ? 1 : -1
          }
          return 0
        })
      }

      const containerMap = {}
      const pmServiceMap = {}
      const helmServiceMap = {}
      const containerNames = []
      for (const group of template.services) {
        for (const ser of group) {
          if (ser.type === 'k8s') {
            containerMap[ser.service_name] = containerMap[ser.service_name] || {}
            containerMap[ser.service_name][ser.type] = ser
            ser.picked = true
            const containers = ser.containers
            if (containers) {
              for (const con of containers) {
                containerNames.push(con.name)
                Object.defineProperty(con, 'defaultImage', {
                  value: con.image,
                  enumerable: false,
                  writable: false
                })
              }
            }
          } else if (ser.type === 'pm') {
            pmServiceMap[ser.service_name] = pmServiceMap[ser.service_name] || {}
            pmServiceMap[ser.service_name][ser.type] = ser
          } else if (ser.type === 'helm') {
            helmServiceMap[ser.service_name] = helmServiceMap[ser.service_name] || {}
            helmServiceMap[ser.service_name][ser.type] = ser
          }
        }
      }
      this.projectConfig.services = template.services
      this.containerMap = containerMap
      this.pmServiceMap = pmServiceMap
      this.helmServiceMap = helmServiceMap
      imagesAPI(uniq(containerNames)).then((images) => {
        if (images) {
          for (const image of images) {
            image.full = `${image.host}/${image.owner}/${image.name}:${image.tag}`
          }
          this.imageMap = this.makeMapOfArray(images, 'name')
          if (!this.rollbackMode) {
            this.quickSelection = 'latest'
          }
        }
      })
    },
    makeMapOfArray (arr, namePropName) {
      const map = {}
      for (const obj of arr) {
        if (!map[obj[namePropName]]) {
          map[obj[namePropName]] = [obj]
        } else {
          map[obj[namePropName]].push(obj)
        }
      }
      return map
    },
    checkImgSelected (container_img_selected) {
      const containerNames = []
      for (const service in container_img_selected) {
        for (const containername in container_img_selected[service]) {
          if (container_img_selected[service][containername] === '') {
            containerNames.push(containername)
          }
        }
      }
      this.unSelectedImgContainers = containerNames
      return containerNames
    },
    mapImgToprojectConfig (product_tpl, container_img_selected) {
      for (const service_con_img in container_img_selected) {
        for (const container in container_img_selected[service_con_img]) {
          product_tpl.services.forEach(service_group => {
            service_group.forEach(service => {
              service.containers.forEach((con, index_con) => {
                if (con.name === container) {
                  service.containers[index_con] = {
                    name: con.name,
                    image: container_img_selected[service.service_name][con.name]
                  }
                }
              })
            })
          })
        }
      }
    },
    addRenderKey () {
      if (this.addKeyData[0].key !== '') {
        this.$refs.addKeyForm.validate(valid => {
          if (valid) {
            this.projectConfig.vars.push(this.$utils.cloneObj(this.addKeyData[0]))
            this.addKeyData[0].key = ''
            this.addKeyData[0].value = ''
            this.$refs.addKeyForm.resetFields()
            this.$refs.addValueForm.resetFields()
          } else {
            return false
          }
        })
      }
    },
    deleteRenderKey (index, state) {
      if (state === 'present') {
        this.$confirm('该 Key 被产品服务模板引用，确定删除', '提示', {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        }).then(() => {
          this.projectConfig.vars.splice(index, 1)
        }).catch(() => {
          this.$message({
            type: 'info',
            message: '已取消删除'
          })
        })
      } else {
        this.projectConfig.vars.splice(index, 1)
      }
    },
    startDeploy () {
      if (this.projectConfig.source === 'versionBack') {
        this.projectConfig.source = 'system'
      }
      const selectType = this.projectConfig.source
      const projectType = this.deployType
      if (projectType === 'k8s' && selectType === 'system') {
        this.deployEnv()
      } else if (projectType === 'helm' && selectType === 'system') {
        this.deployHelmEnv()
      } else if (selectType === 'external') {
        this.loadHosting()
      }
    },
    changeCluster (clusterId) {
      const source = this.projectConfig.source
      if (source === 'external') {
        productHostingNamespaceAPI(clusterId).then((res) => {
          this.hostingNamespace = res
        })
      }
    },
    changeCreateMethodWhenServiceEmpty () {
      this.projectConfig.source = 'external'
      this.changeCreateMethod('external')
    },
    changeCreateMethod (source) {
      const clusterId = this.projectConfig.cluster_id
      if (this.selection) {
        this.getTemplateAndImg()
      }
      this.selection = ''
      if (source === 'external') {
        productHostingNamespaceAPI(clusterId).then((res) => {
          this.hostingNamespace = res
        })
      }
    },
    loadHosting () {
      this.$refs['create-env-ref'].validate((valid) => {
        if (valid) {
          const payload = this.$utils.cloneObj(this.projectConfig)
          payload.services = []
          payload.vars = []
          payload.source = 'external'
          const envType = 'test'
          this.startDeployLoading = true
          createProductAPI(payload, envType).then((res) => {
            const envName = payload.env_name
            this.startDeployLoading = false
            this.$message({
              message: '创建环境成功',
              type: 'success'
            })
            this.$router.push(`/v1/projects/detail/${this.projectName}/envs/detail?envName=${envName}`)
          }, () => {
            this.startDeployLoading = false
          })
        }
      })
    },
    deployEnv () {
      const picked2D = []
      const picked1D = []
      this.$refs['create-env-ref'].validate((valid) => {
        if (valid) {
          // 同名至少要选一个
          for (const name in this.containerMap) {
            let atLeastOnePicked = false
            const typeServiceMap = this.containerMap[name]
            for (const type in typeServiceMap) {
              const service = typeServiceMap[type]
              if (service.type === 'k8s' && service.picked) {
                atLeastOnePicked = true
              } else if (service.type === 'pm') {
                // 物理机默认设置勾选
                atLeastOnePicked = true
              }
            }
            if (!atLeastOnePicked) {
              this.$message.warning(`每个服务至少要选择一种，${name} 未勾选`)
              return
            }
          }

          for (const group of this.projectConfig.services) {
            for (const ser of group) {
              if (ser.picked) {
                picked1D.push(ser)
              }
              const containers = ser.containers
              if (containers && ser.picked && ser.type === 'k8s') {
                for (const con of ser.containers) {
                  if (!con.image) {
                    this.$message.warning(`${con.name}未选择镜像`)
                    return
                  }
                }
              } else if (ser.type === 'pm') {
                ser.env_configs = [{ env_name: this.projectConfig.env_name, host_ids: ser.host_ids }]
                delete ser.host_ids
              }
            }
          }
          picked2D.push(picked1D)
          const payload = this.$utils.cloneObj(this.projectConfig)
          payload.source = 'spock'
          const envType = 'test'
          this.startDeployLoading = true
          createProductAPI(payload, envType).then((res) => {
            const envName = payload.env_name
            this.startDeployLoading = false
            this.$message({
              message: '创建环境成功',
              type: 'success'
            })
            this.$router.push(`/v1/projects/detail/${this.projectName}/envs/detail?envName=${envName}`)
          }, () => {
            this.startDeployLoading = false
          })
        } else {
          console.log('not-valid')
        }
      })
    },
    deployHelmEnv () {
      this.$refs['create-env-ref'].validate((valid) => {
        if (valid) {
          for (let index = 0; index < this.helmCharts.length; index++) {
            const chart = this.helmCharts[index]
            if (!chart.chart_version) {
              this.$message.warning(`${chart.service_name} 未选择版本`)
              return
            }
          }
          this.projectConfig.chart_infos = this.helmCharts
          const payload = this.$utils.cloneObj(this.projectConfig)
          const envType = 'test'
          payload.source = 'helm'
          this.startDeployLoading = true
          createProductAPI(payload, envType).then((res) => {
            const envName = payload.env_name
            this.startDeployLoading = false
            this.$message({
              message: '创建环境成功',
              type: 'success'
            })
            this.$router.push(`/v1/projects/detail/${this.projectName}/envs/detail?envName=${envName}`)
          }, () => {
            this.startDeployLoading = false
          })
        }
      })
    },
    goBack () {
      this.$router.back()
    },
    createHost () {
      this.$router('/v1/system/host')
    },
    editHelmValue (index, row) {
      this.showHelmVarEdit = true
      this.$set(this.currentEditHelmYaml, 'index', index)
      this.$set(this.currentEditHelmYaml, 'value', row.values_yaml)
    },
    saveHelmValue (index, value) {
      this.$set(this.helmCharts[index], 'values_yaml', value)
      this.showHelmVarEdit = false
    }
  },
  watch: {
    quickSelection (select) {
      for (const group of this.projectConfig.services) {
        for (const ser of group) {
          ser.picked = (ser.type === 'k8s' && (select === 'latest' || select === 'default'))
          const containers = ser.containers
          if (containers) {
            for (const con of ser.containers) {
              if (select === 'latest') {
                if (this.imageMap[con.name]) {
                  con.image = this.imageMap[con.name][0].full
                } else {
                  con.image = con.defaultImage
                }
              }
              if (select === 'default') {
                con.image = con.defaultImage
              }
            }
          }
        }
      }
    }
  },
  created () {
    bus.$emit('set-topbar-title', { title: '', breadcrumb: [{ title: '项目', url: `/v1/projects/detail/${this.projectName}` }, { title: `${this.projectName}`, url: `/v1/projects/detail/${this.projectName}` }, { title: '集成环境', url: '' }, { title: '创建', url: '' }] })
    this.getVersionList()
    this.projectConfig.product_name = this.projectName
    this.getTemplateAndImg()
    this.checkProjectFeature()
    this.getCluster()
    getHostListAPI().then((res) => {
      this.allHost = res
    })
  },
  components: {
    editor: aceEditor
  }
}
</script>

<style lang="less">
.create-product-detail-container {
  position: relative;
  flex: 1;
  padding: 15px 20px;
  overflow: auto;
  font-size: 13px;

  .helm-yaml-drawer {
    .el-drawer__header {
      margin-bottom: 10px;

      :first-child {
        &:focus {
          outline: none;
        }
      }
    }

    .el-drawer__body {
      padding: 20px;
    }

    .helm-yaml-drawer__content {
      flex: 1;
    }

    .helm-yaml-drawer__footer {
      display: flex;
      margin-top: 15px;
    }

    .helm-yaml-drawer__footer button {
      flex: 1;
    }
  }

  .module-title h1 {
    margin-bottom: 30px;
    font-weight: 200;
    font-size: 1.5rem;
  }

  .btn {
    display: inline-block;
    min-width: 87px;
    height: 30px;
    margin: 0 auto 38px;
    padding: 0 8px;
    font-weight: 500;
    font-size: 12px;
    line-height: 30px;
    line-height: 32px;
    white-space: nowrap;
    border: 1px solid transparent;
    border-radius: 4px;
    cursor: pointer;
    transition: all 0.15s;
  }

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

  .btn-mute {
    color: rgba(94, 97, 102, 0.8);
    background-color: transparent;
    border-color: rgba(94, 97, 102, 0.4);

    &:hover {
      color: rgba(94, 97, 102, 0.8);
      background-color: transparent;
      border-color: rgba(94, 97, 102, 0.4);
    }

    &[disabled] {
      color: rgba(94, 97, 102, 0.8);
      background-color: transparent;
      border-color: rgba(94, 97, 102, 0.4);
      cursor: not-allowed;
    }
  }

  .box-card,
  .box-card-service {
    margin-top: 25px;
    margin-bottom: 25px;
    border: none;
    box-shadow: none;

    .item {
      .item-name {
        margin: 10px 0;
      }

      .el-row {
        margin-bottom: 15px;
      }

      .img-tooltip {
        color: #5a5e66;
        font-size: 15px;

        &:hover {
          color: #1989fa;
          cursor: pointer;
        }
      }

      .img-select {
        width: 140px;
      }
    }
  }

  .el-card__header {
    position: relative;
    box-sizing: border-box;
    padding-top: 10px;
    padding-bottom: 10px;
    padding-left: 0;
    border-bottom: none;

    &::before {
      position: absolute;
      bottom: 0;
      left: 0;
      width: 400px;
      height: 1px;
      border-bottom: 1px solid #eee;
      content: "";
    }
  }

  .el-collapse-item__header {
    padding-left: 0;
  }

  .no-resources {
    padding: 45px;
    border-style: hidden;
    border-radius: 4px;
    border-collapse: collapse;
    box-shadow: 0 0 0 2px #f1f1f1;

    img {
      display: block;
      width: 360px;
      height: 360px;
      margin: 10px auto;
    }

    .description {
      margin: 16px auto;
      text-align: center;

      p {
        color: #8d9199;
        font-size: 15px;
      }
    }
  }

  .create-footer {
    position: fixed;
    bottom: 0;
    z-index: 5;
    -webkit-box-sizing: border-box;
    box-sizing: border-box;
    width: 800px;
    padding: 15px 60px 10px 0;
    text-align: left;
    background-color: #fff;
    border-top: 1px solid #fff;

    .grid-content {
      min-height: 36px;
      border-radius: 4px;

      .description {
        line-height: 36px;

        p {
          margin: 0;
          color: #676767;
          font-size: 16px;
          line-height: 36px;
          text-align: left;
        }
      }

      .deploy-loading {
        width: 100px;
        margin-left: 70px;
        line-height: 36px;
        text-align: center;

        div {
          display: inline-block;
          width: 8px;
          height: 8px;
          margin-right: 4px;
          background-color: #1989fa;
          border-radius: 100%;
          animation: sk-bouncedelay 1.4s infinite ease-in-out both;
        }

        .spinner__item1 {
          animation-delay: -0.6s;
        }

        .spinner__item2 {
          animation-delay: -0.4s;
        }

        .spinner__item3 {
          animation-delay: -0.2s;
        }

        @keyframes sk-bouncedelay {
          0%,
          80%,
          100% {
            -webkit-transform: scale(0);
            transform: scale(0);
            opacity: 0;
          }

          40% {
            -webkit-transform: scale(1);
            transform: scale(1);
            opacity: 1;
          }
        }
      }
    }
  }

  .el-input__inner {
    width: 250px;
  }

  .el-form-item__label {
    text-align: left;
  }

  .env-form {
    display: flex;

    .el-form-item {
      width: 50%;
      margin-right: 0;
      margin-bottom: 0;
    }
  }

  .second-title {
    color: #606266;
    font-size: 14px;
  }

  .small-title {
    color: #969799;
    font-size: 12px;
  }

  .service-filter {
    margin-left: 56px;
    color: #409eff;

    .el-input__inner {
      color: #409eff;
      border-color: #8cc5ff;

      &::placeholder {
        color: #8cc5ff;
      }
    }
  }

  .el-tag {
    background-color: rgba(64, 158, 255, 0.2);
  }

  .service-form {
    margin: 10px 0 0 0;
    padding-left: 10px;

    .el-form-item {
      margin-bottom: 10px;
    }

    .group {
      margin-top: 10px;
      padding: 10px;
      box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
    }
  }

  .service {
    display: flex;
  }

  .service-block {
    /* width: 50%; */
    margin: 10px 30px 0 0;

    .el-checkbox {
      font-size: 24px;

      .el-checkbox__input {
        height: 22px;
      }
    }
  }

  .container-images {
    margin: 5px 0 0 0;
    padding: 10px 10px 0 10px;
    border: 1px solid #ddd;
    border-radius: 5px;
  }

  .ops {
    margin-top: 25px;
  }

  .kv-container {
    .el-table {
      .unused {
        background: #e6effb;
      }

      .present {
        background: #fff;
      }

      .new {
        background: oldlace;
      }
    }

    .el-table__row {
      .cell {
        span {
          font-weight: 400;
        }

        .operate {
          font-size: 1.12rem;

          .delete {
            color: #ff1949;
          }
        }
      }
    }

    .render-value {
      display: block;
      max-width: 100%;
      overflow: hidden;
      white-space: nowrap;
      text-overflow: ellipsis;
    }

    .add-key-container {
      .el-form-item__label {
        display: none;
      }

      .el-form-item {
        margin-bottom: 15px;
      }
    }

    .add-kv-btn {
      display: inline-block;
      margin-top: 10px;
      margin-left: 5px;

      i {
        padding-right: 4px;
        color: #5e6166;
        color: #1989fa;
        font-size: 14px;
        line-height: 14px;
        cursor: pointer;
      }
    }
  }
}
</style>
