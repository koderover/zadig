<template>
  <div class="service-container">
    <el-dialog title="是否更新对应环境？"
               :append-to-body="true"
               :visible.sync="updateEnvDialogVisible"
               width="40%">
      <el-checkbox-group v-model="selectedEnvs">
        <el-checkbox v-for="(env,index) in envNameList"
                     :key="index"
                     :label="env.envName"></el-checkbox>
      </el-checkbox-group>
      <span slot="footer"
            class="dialog-footer">
        <el-button size="small"
                   type="primary"
                   @click="autoUpgradeEnv">确 定</el-button>
        <el-button size="small"
                   @click="updateEnvDialogVisible = false">跳过</el-button>

      </span>
    </el-dialog>
    <!--start of workspace-tree-dialog-->
    <el-dialog :append-to-body="true"
               :visible.sync="workSpaceModalVisible"
               width="60%"
               title="请选择要同步的文件或文件目录"
               class="fileTree-dialog">
      <gitfile-tree ref="worktree"
                    :codehostId="source.codehostId"
                    :repoName="source.repoName"
                    :repoOwner="source.repoOwner"
                    :branchName="source.branchName"
                    :remoteName="source.remoteName"
                    :gitType="source.gitType"
                    @getPreloadServices="getPreloadServices"
                    :showTree="workSpaceModalVisible"></gitfile-tree>
    </el-dialog>
    <!--end of workspace-tree-dialog-->
    <el-dialog title="请选择代码库平台"
               @close="closeSelectRepo"
               center
               :append-to-body="true"
               :close-on-click-modal="false"
               custom-class="dialog-source"
               :visible.sync="dialogImportFileVisible">
      <div class="from-code-container">

        <el-form :model="source"
                 :rules="sourceRules"
                 ref="sourceForm"
                 label-width="140px">
          <el-form-item label="托管平台"
                        prop="codehostId"
                        :rules="{required: true, message: '平台不能为空', trigger: 'change'}">
            <el-select v-model="source.codehostId"
                       size="small"
                       style="width:100%"
                       placeholder="请选择托管平台"
                       @change="getRepoOwnerById(source.codehostId)"
                       filterable>
              <el-option v-for="(host,index) in allCodeHosts"
                         :key="index"
                         :label="`${host.address} ${host.type==='github'?'('+host.namespace+')':''}`"
                         :value="host.id">{{`${host.address}
                    ${host.type==='github'?'('+host.namespace+')':''}`}}</el-option>
            </el-select>
          </el-form-item>
          <el-form-item label="代码库拥有者"
                        prop="repoOwner"
                        :rules="{required: true, message: '代码库拥有者不能为空', trigger: 'change'}">
            <el-select v-model="source.repoOwner"
                       size="small"
                       style="width:100%"
                       @change="getRepoNameById(source.codehostId,source.repoOwner)"
                       @clear="clearRepoOwner"
                       remote
                       :remote-method="searchRepoOwner"
                       :loading="searchRepoOwnerLoading"
                       allow-create
                       clearable
                       placeholder="请选择代码库拥有者"
                       filterable>
              <el-option v-for="(repo,index) in codeInfo['repoOwners']"
                         :key="index"
                         :label="repo.name"
                         :value="repo.name">
              </el-option>
            </el-select>
          </el-form-item>
          <template>
            <el-form-item label="代码库名称"
                          prop="repoName"
                          :rules="{required: true, message: '名称不能为空', trigger: 'change'}">
              <el-select @change="getBranchInfoById(source.codehostId,source.repoOwner,source.repoName)"
                         @clear="clearRepoName"
                         v-model="source.repoName"
                         remote
                         :remote-method="searchRepoName"
                         :loading="searchRepoNameLoading"
                         style="width:100%"
                         allow-create
                         clearable
                         size="small"
                         placeholder="请选择代码库"
                         filterable>
                <el-option v-for="(repo,index) in codeInfo['repos']"
                           :key="index"
                           :label="repo.name"
                           :value="repo.name">
                </el-option>
              </el-select>
            </el-form-item>
            <el-form-item label="分支"
                          prop="branchName"
                          :rules="{required: true, message: '分支不能为空', trigger: 'change'}">
              <el-select v-model="source.branchName"
                         placeholder="请选择"
                         style="width:100%"
                         size="small"
                         filterable
                         allow-create
                         clearable>
                <el-option v-for="(branch,branch_index) in codeInfo['branches']"
                           :key="branch_index"
                           :label="branch.name"
                           :value="branch.name">
                </el-option>
              </el-select>
            </el-form-item>
            <el-form-item prop="path"
                          label="文件路径："
                          :rules="{required: true, message: '请选择文件', trigger: 'change'}">
              {{ source.path}}
              <el-button v-if="showSelectPath"
                         @click="openFileTree"
                         :disabled="!showSelectFileBtn"
                         type="primary"
                         plain
                         size="mini"
                         round>选择文件目录</el-button>
              <span v-if="disabledReload"
                    class="preload-error">当前服务名称和选中的文件夹名称不符，请重新选择</span>
              <div class="preload-container"
                   v-if="source.services && source.services.length > 0">
                <span class="contains">
                  包含服务:
                </span>
                <span v-for="(service,index) in source.services"
                      :key="index"
                      class="service-name">
                  {{service}}
                </span>
              </div>

            </el-form-item>
          </template>

        </el-form>
      </div>
      <span slot="footer"
            class="dialog-footer">
        <el-button size="small"
                   :loading="importLoading"
                   :disabled="disabledReload"
                   @click="loadRepoService()"
                   plain>加载</el-button>
      </span>
    </el-dialog>
    <div class="menu-container">
      <el-row>
        <el-col :span="10">
          <div class="source-dropdown">
            <el-radio-group v-model="mode"
                            size="mini">
              <el-tooltip effect="dark"
                          content="服务管理"
                          placement="top">
                <el-radio-button label="edit">
                  <i class="iconfont iconiconlog"> </i>
                </el-radio-button>
              </el-tooltip>
              <el-tooltip effect="dark"
                          content="服务编排"
                          placement="top">
                <el-radio-button label="arrange">
                  <i class="iconfont iconfuwu"></i>
                </el-radio-button>
              </el-tooltip>
            </el-radio-group>
          </div>
        </el-col>
        <el-col :span="14"
                class="text-right">
          <div style="line-height:32px">
            <el-tooltip effect="dark"
                        content="创建服务"
                        placement="top">
              <el-button v-if="deployType==='k8s'"
                         size="mini"
                         icon="el-icon-plus"
                         @click="createService('platform')"
                         plain
                         circle>
              </el-button>
            </el-tooltip>
            <el-tooltip effect="dark"
                        content="仓库托管"
                        placement="top">
              <el-button v-if="deployType==='k8s'"
                         size="mini"
                         @click="createService('repo')"
                         icon="iconfont icon icongit"
                         plain
                         circle></el-button>
            </el-tooltip>
          </div>
        </el-col>
      </el-row>
    </div>
    <div class="tree-container">
      <keep-alive>
        <el-tree v-if="mode==='edit'"
                 ref="serviceTree"
                 :data="filteredServices"
                 :show-checkbox="false"
                 node-key="service_name"
                 @node-click="selectService"
                 default-expand-all
                 highlight-current
                 check-on-click-node
                 :indent="0"
                 :expand-on-click-node="false">
          <span @mouseover="setHovered(data.service_name)"
                @mouseleave="unsetHovered(data.service_name)"
                class="service-mgr-tree-node"
                slot-scope="{ node, data }">
            <span class="service-status"
                  :class="data.status"></span>
            <i v-if="data.type==='k8s'"
               class="service-type iconfont iconrongqifuwu"></i>
            <el-tooltip v-if="data.type!=='kind' && data.visibility==='public' && data.product_name!==projectName"
                        effect="light"
                        placement="top">
              <div slot="content">
                <span>
                  {{`服务名：${data.service_name}`}}
                </span>
                <span><br />{{`所属项目：${data.product_name}`}}</span>
              </div>
              <div class="tree-service-name"
                   :class="{'kind':data.type==='kind'?true:false}">
                <span class="label">{{node.label}}</span>
              </div>
            </el-tooltip>
            <span v-else
                  class="tree-service-name"
                  :class="{'kind':data.type==='kind'?true:false}">
              <span class="label">{{node.label}}</span>
            </span>
            <template>
              <el-tag v-if="data.visibility==='public'"
                      type="info"
                      effect="plain"
                      class="operation-container"
                      size="mini">共享
              </el-tag>
              <el-button v-if="data.status==='named' && data.type==='k8s'"
                         type="text"
                         size="mini"
                         class="operation-container"
                         icon="el-icon-edit-outline"
                         @click="() => reEditServiceName(node, data)">
              </el-button>
              <span v-if="showHover[data.service_name]"
                    class="operation-container">
                <el-button v-if="(data.product_name===projectName||data.status === 'named')"
                           type="text"
                           size="mini"
                           icon="el-icon-close"
                           @click="() => deleteService(node, data)">
                </el-button>
                <el-button v-else-if="data.product_name!==projectName && data.type ==='k8s'"
                           type="text"
                           size="mini"
                           icon="el-icon-close"
                           @click="() => deleteSharedService(node, data)">
                </el-button>
                <el-button v-if="data.source && (data.source === 'gerrit'||data.source === 'gitlab'||data.source==='github') && data.type==='k8s' && data.product_name=== projectName "
                           type="text"
                           size="mini"
                           icon="el-icon-refresh"
                           @click="() => refreshService(node, data)">
                </el-button>
              </span>

            </template>

          </span>
        </el-tree>
      </keep-alive>
      <div v-if="showNewServiceInput && mode==='edit'"
           class="add-new-service">
        <el-form :model="service"
                 :rules="serviceRules"
                 ref="newServiceNameForm"
                 @submit.native.prevent>
          <el-form-item label=""
                        prop="newServiceName">
            <span class="service-status new"></span>
            <i class="service-type iconfont iconrongqifuwu"></i>
            <el-input v-model="service.newServiceName"
                      size="mini"
                      autofocus
                      ref="serviceNamedRef"
                      @blur="inputServiceNameDoneWhenBlur"
                      @keyup.enter.native="inputServiceNameDoneWhenEnter"
                      placeholder="请输入服务名称"></el-input>
          </el-form-item>

        </el-form>
      </div>
      <el-tree v-if="mode==='arrange' && deployType === 'k8s'"
               :data="serviceGroup"
               :show-checkbox="false"
               node-key="id"
               draggable
               :allow-drop="allowDrop"
               :allow-drag="allowDrag"
               @node-drop="handleDrop"
               @node-drag-start="startDrag"
               @node-drag-end="endDrag"
               default-expand-all
               :expand-on-click-node="false">
        <span class="service-mgr-tree-node"
              slot-scope="{ node, data }">
          <span class="service-status"
                :class="data.status"></span>
          <span class="tree-service-name">{{ node.label }}</span>
        </span>
      </el-tree>
      <div v-if="mode==='arrange' && showDragContainer"
           class="add-new-service drag-container">
      </div>
    </div>
    <div v-if="mode==='edit' && filteredSharedServices[0] && filteredSharedServices[0].children.length > 0 && deployType ==='k8s'"
         class="shared-services-container">
      <el-tree ref="serviceSharedTree"
               :data="filteredSharedServices"
               :show-checkbox="false"
               node-key="service_name"
               default-expand-all
               highlight-current
               check-on-click-node
               :indent="0"
               :expand-on-click-node="false">
        <span @mouseover="setHovered(data.service_name)"
              @mouseleave="unsetHovered(data.service_name)"
              class="service-mgr-tree-node"
              slot-scope="{ node, data }">
          <i v-if="data.type==='k8s'"
             class="service-type el-icon-share"></i>
          <el-tooltip v-if="node.label!=='共享服务列表'"
                      effect="light"
                      :content="`所属项目：${data.product_name}`"
                      placement="bottom">
            <span class="tree-service-name shared-list">
              {{`${node.label}`}}
            </span>
          </el-tooltip>
          <span v-else
                class="tree-service-name shared-list">
            {{`${node.label}`}}
          </span>
          <span v-if="data.label!=='共享服务列表' && showHover[data.service_name] "
                class="operation-container">
            <el-button v-if="data.product_name!==projectName"
                       type="text"
                       size="mini"
                       icon="el-icon-plus"
                       @click="() => addSharedService(node, data)">
            </el-button>
          </span>
        </span>
      </el-tree>

    </div>
    <div v-if="mode==='edit'"
         class="search-container">

      <el-input placeholder="搜索服务"
                size="small"
                clearable
                suffix-icon="el-icon-search"
                v-model="searchService">
      </el-input>
    </div>
  </div>

</template>

<script>
import gitfileTree from '@/components/common/gitfile_tree.vue';
import { deleteServiceTemplateAPI, autoUpgradeEnvAPI, getSingleProjectAPI, updateEnvTemplateAPI, getCodeSourceAPI, getRepoOwnerByIdAPI, getRepoNameByIdAPI, getBranchInfoByIdAPI, loadRepoServiceAPI, validPreloadService, getCodeSourceByAdminAPI } from '@api';
import { differenceBy } from 'lodash';
import { mapGetters } from 'vuex';
let validateServiceName = (rule, value, callback) => {
  if (value === '') {
    callback(new Error('请输入服务名称'));
  } else {
    if (!/^[a-z0-9-]+$/.test(value)) {
      callback(new Error('名称只支持小写字母和数字，特殊字符只支持中划线'));
    } else {
      callback();
    }
  }
};
export default {
  props: {
    services: {
      type: Array,
      required: true
    },
    sharedServices: {
      type: Array,
      required: true
    },
    currentServiceYamlKinds: {
      type: Object,
      required: true,
      default: null
    },
    projectInfo: {
      type: Object,
      required: true,
      default: null
    },
    basePath: {
      type: String,
      required: true,
    },
    guideMode: {
      type: Boolean,
      default: false,
      required: false
    }
  },
  data() {
    return {
      mode: 'edit',
      service: {
        newServiceName: ''
      },
      showHover: {},
      searchService: '',
      serviceGroup: [],
      allCodeHosts: [],
      selectedEnvs: [],
      updateEnvDialogVisible: false,
      importLoading: false,
      searchRepoNameLoading: false,
      searchRepoOwnerLoading: false,
      workSpaceModalVisible: false,
      dialogImportFileVisible: false,
      showNewServiceInput: false,
      showDragContainer: false,
      showSelectPath: true,
      disabledReload: false,
      codeInfo: {
        repoOwners: [],
        repos: [],
        branches: []
      },
      source: {
        codehostId: null,
        repoOwner: '',
        repoName: '',
        branchName: '',
        remoteName: '',
        gitType: '',
        services: [],
        path: '',
        isDir: false
      },
      sourceRules: {
        url: [{
          required: true,
          message: '请输入 URL 地址',
          trigger: 'blur'
        },
        {
          type: 'url',
          message: '请输入正确的 URL，包含协议',
          trigger: ['blur', 'change']
        }],
      },
      serviceRules: {
        newServiceName: [
          {
            type: 'string',
            required: true,
            validator: validateServiceName,
            trigger: ['blur', 'change']
          }
        ]

      }
    }
  },

  methods: {
    setHovered(name) {
      this.$nextTick(() => {
        this.$set(this.showHover, name, true);
      });
    },
    unsetHovered(name) {
      this.$nextTick(() => {
        this.$set(this.showHover, name, false);
      });
    },
    addSharedService(node, data) {
      let services = [];
      let payload = this.$utils.cloneObj(this.projectInfo);
      const projectName = this.projectName;
      this.serviceGroup.forEach((order, orderIndex) => {
        if (order.children.length > 0) {
          const serviceStringArray = order.children.map(service => {
            return service.label;
          });
          services.push(serviceStringArray);
        };
      });
      services.push([data.service_name]);
      payload.services = services;
      updateEnvTemplateAPI(projectName, payload).then((res) => {
        this.$message.success('添加共享服务成功');
        this.getServiceGroup();
        this.$emit('onRefreshService');
        this.$emit('onRefreshSharedService');
        this.$emit('update:showNext', true);
      });
    },
    deleteSharedService(node, data) {
      let deleteText = '';
      let title = '确认';
      let services = [];
      let payload = this.$utils.cloneObj(this.projectInfo);
      if (data.type === 'k8s') {
        deleteText = `确定要移除 ${data.service_name} 这个共享服务吗？`;
      }
      this.$confirm(`${deleteText}`, `${title}`, {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        const projectName = this.projectName;
        this.serviceGroup.forEach((order, orderIndex) => {
          if (order.children.length > 0) {
            const filterArray = order.children.filter(service => {
              return service.label !== data.service_name;
            });
            const serviceStringArray = filterArray.map(service => {
              return service.label;
            });
            services.push(serviceStringArray);
          };
        });
        payload.services = services;
        updateEnvTemplateAPI(projectName, payload).then((res) => {
          this.getServiceGroup();
          this.$emit('onRefreshSharedService');
          this.$emit('onRefreshService');
          this.$emit('update:showNext', true);
          this.$message.success('共享服务移除成功');
          if (!this.guideMode) {
            this.updateEnvDialogVisible = true;
          };
        });
      });
    },
    async getProducts() {
      await this.$store.dispatch('getProductListSSE').closeWhenDestroy(this);
    },
    getServiceGroup() {
      this.serviceGroup = [];
      const projectName = this.projectName;
      getSingleProjectAPI(projectName).then((res) => {
        res.services.push([]);
        res.services.forEach((order, orderIndex) => {
          this.serviceGroup.push({
            label: `启动顺序 ${orderIndex}`,
            id: orderIndex,
            children: []
          });
          order.forEach((service, serviceIndex) => {
            this.serviceGroup[orderIndex].children.push({
              label: service,
              id: `${orderIndex}-${serviceIndex}`,
              children: []
            })
          });
        });

      });
    },
    openFileTree() {
      if (this.showSelectFileBtn) {
        this.workSpaceModalVisible = true;
      }
    },
    getPreloadServices(val) {
      this.source.path = val.path;
      this.source.isDir = val.isDir;
      this.source.services = val.services;
      this.workSpaceModalVisible = false;
      const codehostId = this.source.codehostId;
      const repoOwner = this.source.repoOwner;
      const repoName = this.source.repoName;
      const branchName = this.source.branchName;
      const remoteName = this.source.remoteName;
      const serviceName = this.source.serviceName;
      const path = val.path;
      const isDir = this.source.isDir;
      if (serviceName) {
        validPreloadService(codehostId, repoOwner, repoName, branchName, path, serviceName, isDir, remoteName).then((res) => {
          this.disabledReload = false;
        }, (err) => {
          this.disabledReload = true;
        });
      }

    },
    loadRepoService() {
      const codehostId = this.source.codehostId;
      const repoOwner = this.source.repoOwner;
      const repoName = this.source.repoName;
      const branchName = this.source.branchName;
      const remoteName = this.source.remoteName;
      const path = this.source.path;
      const isDir = this.source.isDir;
      const payload = {
        product_name: this.projectName,
        visibility: 'private',
        is_dir: isDir,
        type: 'k8s',
        path: path
      };
      this.$refs['sourceForm'].validate((valid) => {
        if (valid) {
          this.importLoading = true;
          loadRepoServiceAPI(codehostId, repoOwner, repoName, branchName, remoteName, payload).then((res) => {
            this.$emit('onRefreshService');
            this.$emit('onRefreshSharedService');
            this.$emit('update:showNext', true);
            this.getServiceGroup();
            this.importLoading = false;
            this.dialogImportFileVisible = false;
            this.$message({
              message: '服务导入成功',
              type: 'success'
            });
            if (this.source.services && this.source.services.length > 0) {
              const firstServiceName = this.source.services[0];
              this.setServiceSelected(firstServiceName);
              this.$router.replace({ query: { service_name: firstServiceName, rightbar: 'help' } });
            }

          })
        } else {
          return false;
        }
      });

    },
    closeSelectRepo() {
      this.source = {
        codehostId: null,
        repoOwner: '',
        repoName: '',
        branchName: '',
        remoteName: '',
        gitType: ''
      };
      this.showSelectPath = true;
      this.$refs['sourceForm'].resetFields();
    },
    startDrag() {
      this.showDragContainer = true;
    },
    endDrag() {
      this.showDragContainer = false;
    },
    allowDrag(draggingNode) {
      return !draggingNode.data.label.includes('启动顺序');
    },
    allowDrop(draggingNode, dropNode, type) {
      if (dropNode.data.label.includes('启动顺序') && type === 'inner') {
        return true;
      } else if (!dropNode.data.label.includes('启动顺序') && type !== 'inner') {
        return true;
      }
      else if (dropNode.data.label.includes('启动顺序') && type === 'prev') {
        return false;
      }
    },
    handleDrop(draggingNode, dropNode, dropType, ev) {
      let services = [];
      let payload = this.$utils.cloneObj(this.projectInfo);
      const projectName = this.projectName;
      this.serviceGroup.forEach((order, orderIndex) => {
        if (order.children.length > 0) {
          const serviceStringArray = order.children.map(service => {
            return service.label;
          });
          services.push(serviceStringArray);
        };
      });
      payload.services = services;
      updateEnvTemplateAPI(projectName, payload).then((res) => {
      });
    },
    async createService(cmd) {
      const res = await getCodeSourceByAdminAPI(1);
      if (cmd && this.deployType === 'k8s') {
        if (cmd === 'platform') {
          this.showNewServiceInput = true;
          this.mode = 'edit';
          this.$nextTick(() => {
            this.$refs.serviceNamedRef.focus();
          });
        }
        else if (cmd === 'repo') {
          if (res && res.length > 0) {
            this.dialogImportFileVisible = true;
            this.showNewServiceInput = false;
            this.mode = 'edit';
            this.showSelectPath = true;
            this.disabledReload = false;
            getCodeSourceAPI().then((res) => {
              this.allCodeHosts = res.filter(element => {
                return element;
              });
            });
          } else {
            this.$emit('onAddCodeSource', true);
          }
        }
      }

    },
    inputServiceNameDoneWhenBlur() {
      this.$refs['newServiceNameForm'].validate((valid) => {
        if (valid) {
          const val = this.service.newServiceName;
          const node = this.$refs.serviceTree.getNode(val);
          if (!node) {
            const data = {
              label: val,
              status: 'named',
              service_name: val,
              type: this.deployType ? this.deployType : 'k8s',
              visibility: 'private'
            };
            this.services.push(data);
            this.setServiceSelected(data.service_name);
            this.$router.replace({ query: { service_name: data.service_name, rightbar: 'help' } });
            this.$emit('onSelectServiceChange', data);
            this.showNewServiceInput = false;
            this.service.newServiceName = '';
          }
        }
      });
    },
    inputServiceNameDoneWhenEnter() {
      this.$refs['newServiceNameForm'].validate((valid) => {
        if (valid) {
          const val = this.service.newServiceName;
          const node = this.$refs.serviceTree.getNode(val);
          let data = null;
          if (!node) {
            data = {
              label: val,
              status: 'named',
              service_name: val,
              type: this.deployType ? this.deployType : 'k8s',
              visibility: 'private'
            };
          }
          else {
            data = {
              label: 'untitled',
              status: 'named',
              service_name: 'untitled',
              type: this.deployType ? this.deployType : 'k8s',
              visibility: 'private'
            };
          }
          if (!node) {
            this.services.push(data);
          }
          this.setServiceSelected(data.service_name);
          this.$router.replace({ query: { service_name: data.service_name, rightbar: 'help' } });
          this.$emit('onSelectServiceChange', data);
          this.showNewServiceInput = false;
          this.service.newServiceName = '';
        }
      });
    },
    reEditServiceName(node, data) {
      const service = this.$utils.cloneObj(data);
      this.$nextTick(() => {
        this.$refs.serviceTree.remove(node);
      });
      const index = this.filteredServices.findIndex(d => d.service_name === service.service_name);
      this.services.splice(index, 1);
      this.showNewServiceInput = true;
      this.service.newServiceName = service.service_name;
      this.$nextTick(() => {
        this.$refs.serviceNamedRef.focus();
      });
    },
    getInitRepoInfo(source) {
      const codehostId = source.codehostId;
      const repoOwner = source.repoOwner;
      const repoName = source.repoName;
      this.$set(this, 'codeInfo', {
        repoOwners: [],
        repos: [],
        branches: []
      });
      getCodeSourceAPI().then((res) => {
        this.allCodeHosts = res.filter(element => {
          return element;
        });
      });
      getRepoOwnerByIdAPI(codehostId).then((res) => {
        this.$set(this.codeInfo, 'repoOwners', res);
        const item = (this.codeInfo.repoOwners.find(item => { return item.path === repoOwner }));
        const type = item ? item['kind'] : 'group';
        getRepoNameByIdAPI(codehostId, type, encodeURI(repoOwner)).then((res) => {
          this.$set(this.codeInfo, 'repos', res);
        });
      });
      getBranchInfoByIdAPI(codehostId, repoOwner, repoName).then((res) => {
        this.$set(this.codeInfo, 'branches', res);
      })
    },
    searchRepoName(query) {
      this.searchRepoNameLoading = true;
      const repoOwner = this.source.repoOwner;
      const item = (this.codeInfo.repoOwners.find(item => { return item.path === repoOwner }));
      const type = item ? item['kind'] : 'group';
      const id = this.source.codehostId;
      getRepoNameByIdAPI(id, type, encodeURI(repoOwner), query).then((res) => {
        this.searchRepoNameLoading = false;
        this.$set(this.codeInfo, 'repos', res);
      });
    },
    clearRepoName() {
      const repoOwner = this.source.repoOwner;
      const item = (this.codeInfo.repoOwners.find(item => { return item.path === repoOwner }));
      const type = item ? item['kind'] : 'group';
      const id = this.source.codehostId;
      getRepoNameByIdAPI(id, type, encodeURI(repoOwner)).then((res) => {
        this.$set(this.codeInfo, 'repos', res);
      });
      this.source.branchName = '';
      this.source.path = '';
      this.source.services = [];
      this.$set(this.codeInfo, 'branches', [])
    },
    getRepoNameById(id, repoOwner, key = '') {
      const item = (this.codeInfo.repoOwners.find(item => { return item.path === repoOwner }));
      const type = item ? item['kind'] : 'group';
      this.$refs['sourceForm'].clearValidate();
      if (repoOwner) {
        getRepoNameByIdAPI(id, type, encodeURI(repoOwner), key).then((res) => {
          this.$set(this.codeInfo, 'repos', res);
        });
      }
      this.source.repoName = '';
      this.source.branchName = '';
      this.source.path = '';
      this.source.services = [];

    },
    searchRepoOwner(query) {
      this.searchRepoOwnerLoading = true;
      const id = this.source.codehostId;
      const type = (this.allCodeHosts.find(item => { return item['id'] === id }))['type'];
      if (type === 'github' && query !== '') {
        const items = this.$utils.filterObjectArrayByKey('name', query, this.codeInfo.repoOwners);
        this.$set(this.codeInfo, 'repoOwners', items);
        this.searchRepoOwnerLoading = false;
      }
      else {
        getRepoOwnerByIdAPI(id, query).then((res) => {
          this.$set(this.codeInfo, 'repoOwners', res);
          this.searchRepoOwnerLoading = false;
        });
      }
    },
    clearRepoOwner() {
      const id = this.source.codehostId;
      getRepoOwnerByIdAPI(id).then((res) => {
        this.$set(this.codeInfo, 'repoOwners', res);
      });
      this.source.repoName = '';
      this.source.branchName = '';
      this.source.path = '';
      this.source.services = [];
      this.$set(this.codeInfo, 'repos', []);
      this.$set(this.codeInfo, 'branches', [])
    },
    getRepoOwnerById(id, key = '') {
      const codehost = this.allCodeHosts.find(item => { return item.id === id });
      const type = codehost ? codehost['type'] : 'gitlab';
      this.source['gitType'] = type;
      this.$refs['sourceForm'].clearValidate();
      this.$set(this.codeInfo, 'repoOwners', []);
      this.$set(this.codeInfo, 'repos', []);
      this.$set(this.codeInfo, 'branches', []);
      getRepoOwnerByIdAPI(id, key).then((res) => {
        this.$set(this.codeInfo, 'repoOwners', res);
      });
      this.source.repoOwner = '';
      this.source.repoName = '';
      this.source.branchName = '';
      this.source.path = '';
      this.source.services = [];

    },
    getBranchInfoById(id, repoOwner, repoName) {
      if (repoName && repoOwner) {
        getBranchInfoByIdAPI(id, repoOwner, repoName).then((res) => {
          this.$set(this.codeInfo, 'branches', res);
        });
        this.source.branchName = '';
        this.source.path = '';
        this.source.services = [];
      }
    },
    refreshService(node, data) {
      this.dialogImportFileVisible = true;
      this.source.codehostId = data.codehost_id;
      this.source.repoOwner = data.repo_owner;
      this.source.repoName = data.repo_name;
      this.source.branchName = data.branch_name;
      this.source.path = data.load_path;
      this.source.gitType = data.source;
      this.source.isDir = data.is_dir;
      this.source.serviceName = data.service_name;
      this.getInitRepoInfo(this.source);
      validPreloadService(data.codehost_id, data.repo_owner, data.repo_name, data.branch_name, data.load_path, data.service_name, data.is_dir).then((res) => {
        this.disabledReload = false;
      }, (err) => {
        this.disabledReload = true;
      });
    },
    deleteService(node, data) {
      if (data.status === 'named') {
        const index = this.services.findIndex(d => d.label === data.label);
        this.services.splice(index, 1);
      } else {
        let deleteText = '';
        let title = '确认';
        if (data.type === 'k8s') {
          deleteText = `确定要删除 ${data.service_name} 这个服务吗？`;
        }
        this.$confirm(`${deleteText}`, `${title}`, {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        }).then(() => {
          deleteServiceTemplateAPI(data.service_name, data.type, this.projectName, data.visibility).then(() => {
            this.$message.success('删除成功');
            if (!this.guideMode) {
              this.updateEnvDialogVisible = true;
            };
            this.$emit('onRefreshService');
            this.$emit('onRefreshSharedService');
            this.getServiceGroup();
            const parent = node.parent;
            const children = parent.data.children || parent.data;
            const index = children.findIndex(d => d.id === data.id);
            children.splice(index, 1);
          });
        });
      }
    },
    selectService(data, node, current) {
      if (data.status && data.status === 'new') {
        return
      } else if (data.type === 'kind') {
        const parentService = node.parent.data;
        const routeService = this.$route.query.service_name;
        if (parentService.service_name !== routeService) {
          this.$router.replace({ query: { service_name: parentService.service_name, rightbar: 'var', kind: data.kind } });
          this.$emit('onSelectServiceChange', parentService);
        }
        this.$emit('onJumpToKind', data);
      } else {
        this.$router.replace({ query: { service_name: data.service_name, rightbar: 'var' } });
        this.$emit('onSelectServiceChange', data);
      }
    },
    setServiceSelected(key) {
      this.$nextTick(() => {
        this.$refs.serviceTree.setCurrentKey(key);
      });
    },
    autoUpgradeEnv() {
      const payload = {
        'env_names': this.selectedEnvs
      };
      const projectName = this.projectName;
      autoUpgradeEnvAPI(projectName, payload).then((res) => {
        this.updateEnvDialogVisible = false;
        this.$message({
          message: '更新环境成功',
          type: 'success'
        });
      })
    },
    listenResize() {
      window.screenHeight = document.body.clientHeight
      let serviceTree = this.$refs.serviceTree;
      let serviceSharedTree = this.$refs.serviceSharedTree;
      let screenHeight = this.guideMode ? window.screenHeight - 560 : window.screenHeight - 400;
      this.$nextTick(() => {
        if (serviceSharedTree) {
          serviceSharedTree.$el.style.maxHeight = 150 + 'px';
        };
        if (serviceTree && serviceSharedTree) {
          serviceTree.$el.style.maxHeight = screenHeight + 'px';
        }
        else {
          serviceTree.$el.style.maxHeight = screenHeight + 180 + 'px';
        }
      });
    },
  },
  computed: {
    ...mapGetters([
      'signupStatus', 'productList',
    ]),
    envNameList() {
      let envNameList = [];
      this.productList.forEach(element => {
        if (element.product_name === this.projectName && element.source !== 'external') {
          envNameList.push({
            envName: element.env_name,
          });
        }
      });
      return envNameList;
    },
    deployType() {
      return 'k8s';
    },
    projectName() {
      return this.$route.params.project_name;
    },
    filteredServices() {
      const services = this.$utils.filterObjectArrayByKey('service_name', this.searchService, this.services);
      return services.map((element, index) => {
        element.visibility = element.visibility;
        element.label = element.service_name;
        element.id = index;
        element.children = [];
        return element;
      });
    },
    filteredSharedServices() {
      const services = this.$utils.filterObjectArrayByKey('service_name', this.searchService, differenceBy(this.sharedServices, this.services, 'service_name'));
      return [
        {
          label: '共享服务列表',
          children: services.map((element, index) => {
            element.visibility = 'public';
            element.label = element.service_name;
            element.id = index;
            element.children = [];
            return element;
          })
        }
      ]
    },
    showSelectFileBtn() {
      return (this.source.codehostId && this.source.repoName !== '' && this.source.branchName !== '');
    },
    queryServiceName() {
      return this.$route.query.service_name;
    },

  },
  watch: {
    'filteredServices': {
      handler(val, old_val) {
        this.$nextTick(() => {
          let data = null;
          const serviceInRoute = val.find(d => d.service_name === this.queryServiceName);
          if (serviceInRoute) {
            data = serviceInRoute;
          }
          else {
            data = val[0];
          }
          if (data && !this.showNewServiceInput) {
            this.setServiceSelected(data.service_name);
            this.$router.replace({ query: { service_name: data.service_name, rightbar: 'var' } });
            this.$emit('onSelectServiceChange', data);
          }
        });
        this.$nextTick(() => {
          this.listenResize();
        });
      }
    },
    'filteredSharedServices': {
      handler(val, old_val) {
        this.$nextTick(() => {
          this.listenResize();
        });
      },
      immediate: true
    },
    'currentServiceYamlKinds': {
      handler(val, old_val) {
        this.$nextTick(() => {
          const kinds = val.payload.map(element => {
            return {
              kind: element.kind,
              type: 'kind',
              label: `${element.kind}.yaml`.toLowerCase(),
              service_name: `${element.kind}.yaml`.toLowerCase()
            }
          });
          let node = this.$refs.serviceTree.getNode(val.service_name);
          node.childNodes = [];
          if (node.data.type === 'k8s') {
            kinds.forEach(element => {
              this.$refs.serviceTree.append(element, node);
            });
          }
        });
      },
    }
  },
  created() {
    this.getProducts();
    this.getServiceGroup();
  },
  mounted() {
    window.addEventListener('resize', this.listenResize);
  },
  beforeDestroy() {
    window.removeEventListener("resize", this.listenResize);
  },
  components: {
    gitfileTree
  }
};
</script>

<style lang="less" >
@import "~@assets/css/component/service-tree.less";
.text-right {
  text-align: right;
}
</style>