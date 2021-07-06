<template>
  <div class="from-code-container">
    <el-tabs type="border-card" v-model="tabName">
      <el-tab-pane label="私有库" name="private">
        <el-form
          v-if="tabName === 'private'"
          :model="source"
          :rules="sourceRules"
          ref="sourceForm"
          label-width="140px"
        >
          <el-form-item
            label="托管平台"
            prop="codehostId"
            :rules="{
              required: true,
              message: '平台不能为空',
              trigger: 'change',
            }"
          >
            <el-select
              v-model="source.codehostId"
              size="small"
              style="width: 100%;"
              placeholder="请选择托管平台"
              @change="queryRepoOwnerById(source.codehostId)"
              filterable
            >
              <el-option
                v-for="(host, index) in allCodeHosts"
                :key="index"
                :label="`${host.address} ${
                  host.type === 'github' ? '(' + host.namespace + ')' : ''
                }`"
                :value="host.id"
                >{{
                  `${host.address}
                    ${host.type === 'github' ? '(' + host.namespace + ')' : ''}`
                }}</el-option
              >
            </el-select>
          </el-form-item>
          <el-form-item
            label="代码库拥有者"
            prop="repoOwner"
            :rules="{
              required: true,
              message: '代码库拥有者不能为空',
              trigger: 'change',
            }"
          >
            <el-select
              v-model="source.repoOwner"
              size="small"
              style="width: 100%;"
              @change="getRepoNameById(source.codehostId, source.repoOwner)"
              placeholder="请选择代码库拥有者"
              filterable
            >
              <el-option
                v-for="(repo, index) in codeInfo['repoOwners']"
                :key="index"
                :label="repo.name"
                :value="repo.name"
              >
              </el-option>
            </el-select>
          </el-form-item>
          <template>
            <el-form-item
              label="代码库名称"
              prop="repoName"
              :rules="{
                required: true,
                message: '名称不能为空',
                trigger: 'change',
              }"
            >
              <el-select
                @change="
                  getBranchInfoById(
                    source.codehostId,
                    source.repoOwner,
                    source.repoName
                  )
                "
                v-model.trim="source.repoName"
                remote
                :remote-method="searchProject"
                :loading="searchProjectLoading"
                style="width: 100%;"
                allow-create
                clearable
                size="small"
                placeholder="请选择代码库"
                filterable
              >
                <el-option
                  v-for="(repo, index) in codeInfo['repos']"
                  :key="index"
                  :label="repo.name"
                  :value="repo.name"
                >
                </el-option>
              </el-select>
            </el-form-item>
            <el-form-item
              label="分支"
              prop="branchName"
              :rules="{
                required: true,
                message: '分支不能为空',
                trigger: 'change',
              }"
            >
              <el-select
                v-model.trim="source.branchName"
                placeholder="请选择"
                style="width: 100%;"
                size="small"
                filterable
                allow-create
                clearable
              >
                <el-option
                  v-for="(branch, branch_index) in codeInfo['branches']"
                  :key="branch_index"
                  :label="branch.name"
                  :value="branch.name"
                >
                </el-option>
              </el-select>
            </el-form-item>
            <el-form-item
              label="目录路径："
              :rules="{
                required: true,
                message: '请选择目录',
                trigger: 'change',
              }"
            >
              <span :key="item" v-for="item in selectPath"
                >[{{ item }}]&nbsp;</span
              >
              <el-button
                @click="openFileTree"
                :disabled="!showSelectFileBtn"
                type="primary"
                plain
                size="mini"
                round
                >选择文件目录</el-button
              >
            </el-form-item>
          </template>
          <el-form-item>
            <el-button
              size="small"
              plain
              :loading="loading"
              :disabled="selectPath.length === 0"
              @click="submit"
              >加载
            </el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>
      <el-tab-pane label="公开源" name="public">
        <el-form
          v-if="tabName === 'public'"
          :model="source"
          :rules="sourceRules"
          ref="sourceForm"
          label-width="140px"
        >
          <el-form-item prop="url" label="仓库地址">
            <el-input
              v-model="source.url"
              placeholder="https://github.com/owner/repo"
            ></el-input>
          </el-form-item>
          <el-form-item prop="path" label="文件目录：">
            <span :key="item" v-for="item in selectPath"
              >[{{ item }}]&nbsp;</span
            >
            <el-button
              @click="openFileTree"
              :disabled="!source.url"
              type="primary"
              plain
              size="mini"
              round
              >选择文件目录</el-button
            >
          </el-form-item>
          <el-form-item>
            <el-button size="small" :loading="loading" plain @click="submit">加载 </el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>
    </el-tabs>
    <el-dialog
      :append-to-body="true"
      :visible.sync="workSpaceModalVisible"
      width="60%"
      title="请选择要同步的文件目录"
      class="fileTree-dialog"
    >
      <Gitfile
        v-if="source.codehostId || source.url"
        :codehostId="source.codehostId"
        :repoName="source.repoName"
        :repoOwner="source.repoOwner"
        :branchName="source.branchName"
        :remoteName="source.remoteName"
        :showTree="workSpaceModalVisible"
        :closeFileTree="closeFileTree"
        :type="tabName"
        :url="source.url"
        :changeSelectPath="changeSelectPath"
      />
    </el-dialog>
  </div>
</template>
<script>
import {
  getCodeSourceAPI,
  getRepoNameByIdAPI,
  getRepoOwnerByIdAPI,
  getBranchInfoByIdAPI,
  addHelmChartAPI,
  getRepoFilesAPI,
  getPublicRepoFilesAPI
} from '@api'
import Gitfile from './gitfile_tree'
export default {
  name: 'repo',
  props: {
    value: Boolean,
    currentService: Object
  },
  components: {
    Gitfile
  },
  data () {
    return {
      loading: false,
      tabName: 'private',
      allCodeHosts: [],
      searchProjectLoading: false,
      workSpaceModalVisible: false,
      selectPath: [],
      source: {
        codehostId: null,
        repoOwner: '',
        repoName: '',
        branchName: '',
        remoteName: '',
        gitType: '',
        services: [],
        path: '',
        isDir: false,
        url: null
      },
      sourceRules: {
        url: [
          {
            required: true,
            message: '请输入 URL 地址',
            trigger: 'blur'
          },
          {
            type: 'url',
            message: '请输入正确的 URL，包含协议',
            trigger: ['blur', 'change']
          }
        ]
      },
      codeInfo: {
        repoOwners: [],
        repos: [],
        branches: []
      }
    }
  },
  methods: {
    closeSelectRepo () {
      this.source = {
        codehostId: null,
        repoOwner: '',
        repoName: '',
        branchName: '',
        remoteName: '',
        gitType: ''
      }
      this.selectPath = []
      this.$refs.sourceForm.resetFields()
    },
    async queryCodeSource () {
      const res = await getCodeSourceAPI().catch((error) => console.log(error))
      if (res) {
        this.allCodeHosts = res
      }
    },
    async queryRepoOwnerById (id, key = '') {
      this.source.repoOwner = ''
      this.source.repoName = ''
      this.source.branchName = ''
      const res = await getRepoOwnerByIdAPI(id, key).catch((error) =>
        console.log(error)
      )
      if (res) {
        this.codeInfo.repoOwners = res
      }
    },
    async searchProject (query) {
      this.searchProjectLoading = true
      const repoOwner = this.source.repoOwner
      const item = this.codeInfo.repoOwners.find((item) => {
        return item.path === repoOwner
      })
      const type = item ? item.kind : 'group'
      const id = this.source.codehostId
      const res = await getRepoNameByIdAPI(
        id,
        type,
        encodeURI(repoOwner),
        query
      ).catch((error) => console.log(error))
      if (res) {
        this.codeInfo.repos = res
      }
      this.searchProjectLoading = false
    },
    getRepoNameById (id, repoOwner, key = '') {
      this.source.repoName = ''
      this.source.branchName = ''
      const item = this.codeInfo.repoOwners.find((item) => {
        return item.path === repoOwner
      })
      const type = item ? item.kind : 'group'
      this.$refs.sourceForm.clearValidate()
      getRepoNameByIdAPI(id, type, encodeURI(repoOwner), key).then((res) => {
        this.$set(this.codeInfo, 'repos', res)
      })
    },
    getBranchInfoById (id, repoOwner, repoName) {
      this.source.branchName = ''
      if (repoName && repoOwner) {
        getBranchInfoByIdAPI(id, repoOwner, repoName).then((res) => {
          this.$set(this.codeInfo, 'branches', res)
        })
      }
    },
    openFileTree () {
      this.$refs.sourceForm.validate().then((res) => {
        this.workSpaceModalVisible = true
      })
    },
    closeFileTree () {
      this.dialogVisible = false
      this.$store.dispatch('queryService', {
        projectName: this.$route.params.project_name
      })
      this.$emit('canUpdateEnv')
      this.$emit('triggleAction')
    },
    changeSelectPath (path) {
      this.selectPath = path
      this.workSpaceModalVisible = false
    },
    async addService () {
      const projectName = this.$route.params.project_name
      let payload = {}
      let repoName = null
      if (this.tabName === 'public') {
        repoName = this.source.url.match(/https:\/\/github.com\/.*\/(\S*)/)[1]
        payload = {
          src_path: this.source.url,
          repo_name: repoName,
          file_paths: this.selectPath
        }
      } else {
        payload = {
          codehost_id: this.source.codehostId,
          repo_owner: this.source.repoOwner,
          repo_name: this.source.repoName,
          branch_name: this.source.branchName,
          file_paths: this.selectPath
        }
      }
      const res = await addHelmChartAPI(projectName, payload).catch((error) =>
        console.log(error)
      )
      if (res) {
        this.closeFileTree()
      }
      this.loading = false
    },
    async submit () {
      const path = ''
      this.$refs.sourceForm.validate().then((res) => {
        this.loading = true
        if (this.currentService) {
          if (this.tabName === 'public') {
            getPublicRepoFilesAPI(path, this.source.url).then((res) => {
              this.addService()
            })
          } else {
            getRepoFilesAPI(
              this.source.codehostId,
              this.source.repoOwner,
              this.source.repoName,
              this.source.branchName,
              path,
              'gerrit'
            ).then((res) => {
              this.addService()
            })
          }
        } else {
          this.addService()
        }
      })
    }
  },
  computed: {
    showSelectFileBtn () {
      return (
        this.source.codehostId &&
        this.source.repoName !== '' &&
        this.source.branchName !== ''
      )
    },
    dialogVisible: {
      get: function () {
        return this.value
      },
      set: function (value) {
        this.$emit('input', value)
      }
    }
  },
  watch: {
    value: {
      handler (value) {
        const currentService = this.currentService
        if (value && currentService) {
          if (currentService.src_path) {
            this.tabName = 'public'
          } else {
            this.tabName = 'private'
          }
          this.source.codehostId = currentService.codehost_id
          this.source.branchName = currentService.branch_name
          this.source.repoName = currentService.repo_name
          this.source.repoOwner = currentService.repo_owner
          this.source.url = currentService.src_path
          this.selectPath = [currentService.load_path]
        }
      },
      immediate: true
    }
  },
  mounted () {
    this.queryCodeSource()
  }
}
</script>
<style lang="less" scoped>
.submit-button {
  text-align: center;
}
</style>
