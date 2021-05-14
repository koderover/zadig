<template>
  <div class="">
    <el-form ref="buildRepo"
             :inline="true"
             :model="config"
             class="form-style"
             label-position="top"
             label-width="80px">
      <el-tooltip content="在执行工作流任务时系统根据配置克隆代码到工作目录"
                  placement="right">
        <span class="item-title">{{title?title:'代码信息'}}</span>
      </el-tooltip>
      <el-button v-if="config.repos.length===0"
                 style="padding:0"
                 :size="addBtnMini?'mini':'medium'"
                 @click="addFirstBuildRepo()"
                 type="text">新增</el-button>
      <div v-if="showDivider"
           class="divider item"></div>
      <div v-for="(repo,repo_index) in config.repos"
           :key="repo_index">
        <el-row>
          <el-col :span="showAdvanced || showTrigger ?4:5">
            <el-form-item :label="repo_index === 0 ? (shortDescription?'平台':'托管平台') : ''"
                          :prop="'repos.' + repo_index + '.codehost_id'"
                          :rules="{required: true, message: '平台不能为空', trigger: 'blur'}">
              <el-select v-model="config.repos[repo_index].codehost_id"
                         size="small"
                         placeholder="请选择托管平台"
                         @change="getRepoOwnerById(repo_index,config.repos[repo_index].codehost_id)"
                         filterable>
                <el-option v-for="(host,index) in allCodeHosts"
                           :key="index"
                           :label="`${host.address} ${host.type==='github'?'('+host.namespace+')':''}`"
                           :value="host.id">{{`${host.address}
                    ${host.type==='github'?'('+host.namespace+')':''}`}}</el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="showAdvanced || showTrigger ?4:5">
            <el-form-item :label="repo_index === 0 ?(shortDescription?'拥有者':'代码库拥有者') : ''"
                          :prop="'repos.' + repo_index + '.repo_owner'"
                          :rules="{required: true, message: '拥有者不能为空', trigger: 'blur'}">
              <el-select @change="getRepoNameById(repo_index,config.repos[repo_index].codehost_id,config.repos[repo_index]['repo_owner'])"
                         v-model="config.repos[repo_index]['repo_owner']"
                         remote
                         :remote-method="(query)=>{searchNamespace(repo_index,query)}"
                         @clear="searchNamespace(repo_index,'')"
                         allow-create
                         clearable
                         size="small"
                         placeholder="代码库拥有者"
                         filterable>
                <el-option v-for="(repo_owner,index) in codeInfo[repo_index]['repo_owners']"
                           :key="index"
                           :label="repo_owner.path"
                           :value="repo_owner.path">
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="showAdvanced || showTrigger ?4:5">
            <el-form-item :label="repo_index === 0 ? (shortDescription?'名称':'代码库名称') : ''"
                          :prop="'repos.' + repo_index + '.repo_name'"
                          :rules="{required: true, message: '名称不能为空', trigger: 'blur'}">
              <el-select @change="getBranchInfoById(repo_index,config.repos[repo_index].codehost_id,config.repos[repo_index].repo_owner,config.repos[repo_index].repo_name)"
                         v-model="config.repos[repo_index].repo_name"
                         remote
                         :remote-method="(query)=>{searchProject(repo_index,query)}"
                         @clear="searchProject(repo_index,'')"
                         allow-create
                         clearable
                         size="small"
                         placeholder="请选择代码库"
                         filterable>
                <el-option v-for="(repo,index) in codeInfo[repo_index]['repos']"
                           :key="index"
                           :label="repo.name"
                           :value="repo.name">
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="showAdvanced || showTrigger ?4:5 ">
            <el-form-item :label="repo_index === 0 ? (shortDescription?'分支':'默认分支') : ''"
                          :prop="'repos.' + repo_index + '.branch'"
                          :rules="{required: true, message: '分支不能为空', trigger: 'blur'}">
              <el-select v-model="config.repos[repo_index].branch"
                         placeholder="请选择"
                         size="small"
                         filterable
                         allow-create
                         clearable>
                <el-option v-for="(branch,branch_index) in codeInfo[repo_index]['branches']"
                           :key="branch_index"
                           :label="branch.name"
                           :value="branch.name">
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col v-if="showAdvanced"
                  :span="3">
            <el-form-item :label="repo_index === 0 ? '高级':''">
              <div class="app-operation">
                <el-button type="primary"
                           size="mini"
                           round
                           plain
                           v-if="!showAdvancedSetting[repo_index]"
                           @click="$set(showAdvancedSetting,repo_index,true)">展开
                  &#x3E;</el-button>
                <el-button type="primary"
                           size="mini"
                           round
                           plain
                           v-if="showAdvancedSetting[repo_index]"
                           @click="$set(showAdvancedSetting,repo_index,false)">收起
                  &#x3C;</el-button>
              </div>
            </el-form-item>
          </el-col>
          <el-col v-if="showTrigger"
                  :span="3">
            <el-form-item :label="repo_index === 0 ? '启用触发器': ''">
              <span slot="label">
                <span>启用触发器 </span>
                <el-tooltip effect="dark"
                            content="代码仓库的 Git Pull Request 以及 Git Push 事件会触发工作流执行，可以在后续的配置中进行修改"
                            placement="top">
                  <i class="pointer el-icon-question"></i>
                </el-tooltip>
              </span>
              <el-switch v-model="config.repos[repo_index].enableTrigger">
              </el-switch>
            </el-form-item>
          </el-col>
          <el-col v-if="!showJustOne"
                  :span="showAdvanced || showTrigger ?5:4 ">
            <el-form-item :label="repo_index === 0 ? '操作':''">
              <div class="app-operation">
                <el-button v-if="config.repos.length >= 1"
                           @click="deleteBuildRepo(repo_index)"
                           type="danger"
                           size="small"
                           icon="el-icon-minus"
                           plain
                           circle></el-button>
                <el-button v-if="repo_index===config.repos.length-1"
                           @click="addBuildRepo(repo_index)"
                           type="primary"
                           icon="el-icon-plus"
                           size="small"
                           plain
                           circle></el-button>
              </div>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row v-if="showAdvancedSetting[repo_index]"
                style="background-color: rgb(240, 242, 245);padding:4px">
          <el-col :span="5">
            <el-form-item label="Remote name">
              <el-input v-model="repo.remote_name"
                        size="small"
                        placeholder="请输入"></el-input>
            </el-form-item>
          </el-col>
          <el-col :span="4">
            <el-form-item label="克隆目录名">
              <el-input v-model="repo.checkout_path"
                        size="small"
                        placeholder="请输入"></el-input>
            </el-form-item>
          </el-col>
          <el-col :span="3"
                  style="margin-left: 4px;">
            <el-form-item label="子模块">
              <el-switch v-model="repo.submodules"></el-switch>
            </el-form-item>
          </el-col>
          <el-col :span="4">
            <el-form-item label="默认配置">
              <el-switch v-model="repo.use_default"></el-switch>
            </el-form-item>
          </el-col>
          <el-col v-if="!hidePrimary"
                  :span="3">
            <el-form-item>
              <slot name="label">
                <label class="el-form-item__label">主库
                  <el-tooltip class="item"
                              effect="dark"
                              content="设置为主库后，构建出的镜像 Tag 以及 Tar 包的名称与该库的分支或 PR 信息有关"
                              placement="top">
                    <i class="el-icon-question"></i>
                  </el-tooltip>
                </label>
              </slot>
              <div class="el-form-item__content">
                <el-switch @change="changeToPrimaryRepo(repo_index,repo.is_primary)"
                           v-model="repo.is_primary"></el-switch>
              </div>
            </el-form-item>
          </el-col>
        </el-row>
      </div>
    </el-form>
  </div>
</template>

<script type="text/javascript">
import { getCodeSourceAPI, getRepoOwnerByIdAPI, getRepoNameByIdAPI, getBranchInfoByIdAPI } from '@api';
import { orderBy } from 'lodash';
export default {
  data() {
    return {
      allCodeHosts: [],
      codeInfo: {},
      showAdvancedSetting: {},
      validateName: 'repoSelect',
    };
  },
  props: {
    config: {
      required: true,
      type: Object
    },
    showDivider: {
      required: false,
      type: Boolean,
      default: false
    },
    showAdvanced: {
      required: false,
      type: Boolean,
      default: true
    },
    title: {
      required: false,
      type: String,
      default: '代码信息'
    },
    addBtnMini: {
      required: false,
      type: Boolean,
      default: false
    },
    shortDescription: {
      required: false,
      type: Boolean,
      default: false
    },
    hidePrimary: {
      required: false,
      type: Boolean,
      default: false
    },
    showFirstLine: {
      required: false,
      type: Boolean,
      default: false
    },
    validObj: {
      required: false,
      type: Object,
      default: null
    },
    showTrigger: {
      required: false,
      type: Boolean,
      default: false
    },
    showJustOne: {
      required: false,
      type: Boolean,
      default: false
    },
  },
  methods: {
    validateForm() {
      return new Promise((resolve, reject) => {
        this.$refs['buildRepo'].validate((valid) => {
          if (valid) {
            resolve(true);
          } else {
            reject(false);
          }
        });
      })
    },
    addBuildRepo(index) {
      let repoMeta = {
        codehost_id: '',
        repo_owner: '',
        repo_name: '',
        branch: '',
        checkout_path: '',
        remote_name: 'origin',
        submodules: false
      };
      this.showTrigger && (repoMeta.enableTrigger = false);
      this.validateForm().then(res => {
        if (this.allCodeHosts && this.allCodeHosts.length === 1) {
          const codeHostId = this.allCodeHosts[0].id;
          repoMeta.codehost_id = codeHostId;
          this.getRepoOwnerById(index + 1, codeHostId);
        }
        this.config.repos.push(repoMeta);
        this.$set(this.codeInfo, index + 1, {
          repo_owners: [],
          repos: [],
          branches: []
        });
      }).catch(err => {
        return false;
      })

    },
    addFirstBuildRepo() {
      let repoMeta = {
        codehost_id: '',
        repo_owner: '',
        repo_name: '',
        branch: '',
        checkout_path: '',
        remote_name: 'origin',
        submodules: false
      };
      this.showTrigger && (repoMeta.enableTrigger = false);
      this.$set(this.codeInfo, 0, {
        repo_owners: [],
        repos: [],
        branches: []
      });
      if (this.allCodeHosts && this.allCodeHosts.length === 1) {
        const codeHostId = this.allCodeHosts[0].id;
        repoMeta.codehost_id = codeHostId;
        this.getRepoOwnerById(0, codeHostId);
      }
      this.config.repos.push(repoMeta)
    },
    deleteBuildRepo(index) {
      this.config.repos.splice(index, 1);
    },
    searchNamespace(index, query) {
      const id = this.config.repos[index].codehost_id;
      const codehostType = (this.allCodeHosts.find(item => { return item['id'] === id }))['type'];
      if (codehostType === 'github' && query !== '') {
        const items = this.$utils.filterObjectArrayByKey('name', query, this.codeInfo[index]['origin_repo_owners']);
        this.$set(this.codeInfo[index], 'repo_owners', items);
        if (this.allCodeHosts && this.allCodeHosts.length > 1) {
          this.config.repos[index].repo_owner = '';
          this.config.repos[index].repo_name = '';
          this.config.repos[index].branch = '';
        }
      }
      else {
        this.getRepoOwnerById(index, id, query);
      }
    },
    getRepoNameById(index, id, repo_owner, key = '') {
      const item = (this.codeInfo[index].repo_owners.find(item => { return item.path === repo_owner }));
      const type = item ? item['kind'] : 'group';
      if (repo_owner) {
        getRepoNameByIdAPI(id, type, encodeURI(repo_owner), key).then((res) => {
          this.$set(this.codeInfo[index], 'repos', orderBy(res, ['name']));
          this.$set(this.codeInfo[index], 'origin_repos', orderBy(res, ['name']));
        });
      }
      this.config.repos[index].repo_name = '';
      this.config.repos[index].branch = '';

    },
    getBranchInfoById(index, id, repo_owner, repo_name) {
      if (repo_owner && repo_name) {
        getBranchInfoByIdAPI(id, repo_owner, repo_name).then((res) => {
          this.$set(this.codeInfo[index], 'branches', res);
        });
      }
      this.config.repos[index].branch = '';
    },
    getRepoOwnerById(index, id, key = '') {
      getRepoOwnerByIdAPI(id, key).then((res) => {
        this.$set(this.codeInfo[index], 'repo_owners', orderBy(res, ['name']));
        this.$set(this.codeInfo[index], 'origin_repo_owners', orderBy(res, ['name']));
      });
      if (this.allCodeHosts && this.allCodeHosts.length > 1) {
        this.config.repos[index].repo_owner = '';
        this.config.repos[index].repo_name = '';
        this.config.repos[index].branch = '';
      }
    },
    getInitRepoInfo(repos) {
      repos.forEach((element, index) => {
        const codehostId = element.codehost_id;
        const repoOwner = element.repo_owner;
        const repoName = element.repo_name;
        const branch = element.branch;
        this.$set(this.codeInfo, index, {
          repo_owners: [],
          repos: [],
          branches: []
        });
        if (codehostId) {
          getRepoOwnerByIdAPI(codehostId).then((res) => {
            this.$set(this.codeInfo[index], 'repo_owners', orderBy(res, ['name']));
            this.$set(this.codeInfo[index], 'origin_repo_owners', orderBy(res, ['name']));
            const item = this.codeInfo[index].repo_owners.find(item => { return item.path === repoOwner });
            const type = item ? item['kind'] : 'group';
            getRepoNameByIdAPI(codehostId, type, encodeURI(repoOwner)).then((res) => {
              this.$set(this.codeInfo[index], 'repos', orderBy(res, ['name']));
              this.$set(this.codeInfo[index], 'origin_repos', orderBy(res, ['name']));
            });
          })
          getBranchInfoByIdAPI(codehostId, repoOwner, repoName).then((res) => {
            this.$set(this.codeInfo[index], 'branches', res);
          })
        }

      });
    },
    searchProject(index, query) {
      const id = this.config.repos[index].codehost_id;
      const repo_owner = this.config.repos[index].repo_owner;
      const codehostType = (this.allCodeHosts.find(item => { return item['id'] === id }))['type'];
      if (codehostType === 'github') {
        const items = this.$utils.filterObjectArrayByKey('name', query, this.codeInfo[index]['origin_repos']);
        this.$set(this.codeInfo[index], 'repos', items);
        this.config.repos[index].repo_name = '';
        this.config.repos[index].branch = '';
      }
      else {
        this.getRepoNameById(index, id, repo_owner, query);
      }

    },
    changeToPrimaryRepo(index, val) {
      this.config.repos.forEach((item, item_index) => {
        item.is_primary = false;
        if (index === item_index && val) {
          item.is_primary = true;
        }
      });
    },
  },
  computed: {
    currentOrganizationId() {
      return this.$store.state.login.userinfo.organization.id;
    },
  },
  mounted() {
    const orgId = this.currentOrganizationId;
    getCodeSourceAPI(orgId).then((response) => {
      this.allCodeHosts = response;
    });
    (this.showFirstLine) && this.addFirstBuildRepo();
  },
  watch: {
    config(new_val, old_val) {
      if (new_val.repos.length > 0) {
        this.getInitRepoInfo(new_val.repos);
      }
    },
    'config.repos'(new_val, old_val) {
      if (this.validObj !== null) {
        if (new_val && new_val.length > 0) {
          this.validObj.addValidate({
            name: this.validateName,
            valid: this.validateForm
          })
        } else {
          this.validObj.deleteValidate({
            name: this.validateName
          })
        }
      }
    }
  },
  components: {
  }
};
</script>

<style lang="less" scoped>
.form-style {
  .item-title {
    font-size: 15px;
  }
}
.divider {
  height: 1px;
  background-color: #dfe0e6;
  margin: 5px 0 15px 0;
  width: 100%;
  &.item {
    width: 30%;
  }
}
</style>
