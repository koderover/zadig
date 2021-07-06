<template>
  <div class="content">
    <div
      class="modalContent"
      :style="`z-index: ${zindex}`"
      @click="changeModalStatus(null, false, null, true)"
    >
      <div
        class="modal"
        v-show="modalvalue"
        :style="`left:${position.left}px;top: ${position.top}px`"
      >
        <!-- <div
          class="item"
          v-if="!(currentData && !currentData.type ==='folder')"
          @click="newFile('file')"
        >
          新增文件
        </div>
        <div
          class="item"
          v-if="!(currentData && !currentData.type ==='folder')"
          @click="newFile('folder')"
        >
          新增目录
        </div>
        <div class="item" v-if="currentData" @click="updateFileName">
          重命名
        </div>
        <div class="item" v-if="currentData" @click="deleteFile">删除</div> -->
        <div class="item" v-if="currentData.type && (actionList[currentData.type].includes('deleteServer'))" @click="deleteServer">删除服务</div>
     </div>
    </div>

    <multipane class="custom-resizer" layout="vertical">
      <div class="left">
        <div class="title">
            <el-button  @click="openRepoModal()"
                size="mini"
                icon="el-icon-plus"
                plain
                circle>
            </el-button>
        </div>
        <Folder
          class="folder"
          ref="folder"
          :changeModalStatus="changeModalStatus"
          :laodData="laodData"
          :saveFileName="saveFileName"
          :saveNewFile="saveNewFile"
          :saveNewFolder="saveNewFolder"
          :nodeData="nodeData"
          :expandKey="expandKey"
          :changeExpandFileList="changeExpandFileList"
          :deleteServer="deleteServer"
          :openRepoModal="openRepoModal"
        />
        <div class="bottom">
           <el-button size="small" type="primary" @click="commit" :disabled="!commitCache.length">保存</el-button>
          <el-button size="small" type="primary" :disabled="!updateEnv" @click="update">更新环境</el-button>
        </div>
      </div>
      <multipane-resizer class="resizer1"></multipane-resizer>
      <div class="center">
        <div class="header">
          <PageNav
            ref="pageNav"
            :expandFileList="page.expandFileList"
            :changeExpandFileList="changeExpandFileList"
            :closePageNoSave="closePageNoSave"
            :setData="setData"
            :saveFile="saveFile"
          />
        </div>
        <div class="code" v-if="page.expandFileList.length">
          <component v-if="currentCode.type==='components'" :changeExpandFileList="changeExpandFileList" :currentCode="currentCode" v-bind="currentCode" v-bind:is="currentCode.componentsName"></component>
          <CodeMirror v-if="currentCode.type==='file'"
            :saveFile="saveFile"
            :changeCodeTxtCache="changeCodeTxtCache"
            :currentCode="currentCode"
          />
        </div>
      </div>
      <multipane-resizer class="resizer2" v-if="service && service.length"></multipane-resizer>

      <div :style="{ flexGrow: 1 }" class="right">
        <ServiceAside :changeExpandFileList="changeExpandFileList" ref="aside" slot="aside" /> <!-- 右侧aside -->
      </div>
      </multipane>
     <UpdateHelmEnv v-model="updateHelmEnvDialogVisible"/>
    <el-dialog
      title="请选择导入源"
      :visible.sync="dialogVisible"
      center
      @close="closeSelectRepo"
    >
      <Repo ref="repo" @triggleAction="changeExpandFileList('clear');clearCommitCache()" :currentService="currentService" @canUpdateEnv="updateEnv = true" v-model="dialogVisible" /> <!-- 代码库弹窗 -->
    </el-dialog>
  </div>
</template>
<script>
import Folder from './components/editor/folder'
import PageNav from './components/editor/page_nav'
import CodeMirror from './components/editor/code_mirror'
import Repo from './components/common/repo'
import ServiceAside from './components/helm/aside'
import { cloneDeep } from 'lodash'
import { Multipane, MultipaneResizer } from 'vue-multipane'
import UpdateHelmEnv from './components/common/update_helm_env'
import Build from './components/common/build'
import { deleteServiceTemplateAPI } from '@api'
import { mapState } from 'vuex'

const actionList = {
  file: ['delete'],
  folder: [],
  service: ['deleteServer']
}
const newFileNode = {
  label: null,
  add: true,
  txt: '',
  type: 'file'
}
const newFolderNode = {
  label: null,
  add: true,
  children: [],
  type: 'folder'
}
export default {
  name: 'vscode',
  components: {
    Folder,
    PageNav,
    CodeMirror,
    Multipane,
    MultipaneResizer,
    UpdateHelmEnv,
    Build,
    Repo,
    ServiceAside
  },
  data () {
    return {
      currentService: null,
      actionList: actionList,
      updateHelmEnvDialogVisible: false,
      updateEnv: false,
      dialogVisible: false,
      commitCache: [],
      currentCode: null,
      page: {
        expandFileList: []
      },
      expandKey: [0],
      currentData: {
        children: null
      },
      nodeData: [],
      zindex: 0,
      modalvalue: false,
      position: {
        left: 0,
        top: 0
      }
    }
  },
  methods: {
    closeSelectRepo () {
      this.$refs.repo.closeSelectRepo()
    },
    laodData (data) {
      let path = ''
      if (data.parent) {
        if (data.parent === '/') {
          path = data.parent + data.label
        } else {
          path = data.parent + '/' + data.label
        }
      }
      this.expandKey = []
      const params = {
        serviceName: data.service_name,
        path: path,
        projectName: this.projectName
      }
      this.$store.dispatch('queryFilePath', params).then(res => {
        this.expandKey.push(data.id)
        data.children = res
      })
    },
    setData (obj, data) {
      this[obj] = data
    },
    updateData (obj, key, value) {
      if (Object.prototype.hasOwnProperty.call(this[obj], key)) {
        this[obj][key] = value
      } else {
        this.$set(this[obj], key, value)
      }
    },
    changeCodeTxtCache (code) {
      this.updateData('currentCode', 'isUpdate', true)
      this.updateData('currentCode', 'updated', false)
      this.updateData('currentCode', 'cacheTxt', code)
    },
    closePageNoSave () {
      this.updateData('currentCode', 'isUpdate', false)
      this.updateData('currentCode', 'cacheTxt', '')
    },
    changeExpandFileList (method, item, index) {
      if (method === 'clear') {
        this.page.expandFileList = []
      } else {
        const resIndex = this.page.expandFileList.findIndex(
          (i) => i.id === item.id
        )
        const length = this.page.expandFileList.length
        if (method === 'add') {
          if (resIndex < 0) {
            this.page.expandFileList.push(item)
            this.$refs.pageNav.changePage(length, item)
          } else {
            this.$refs.pageNav.changePage(resIndex, item)
          }
        } else if (method === 'del') {
          if (length > 0) {
            this.page.expandFileList.splice(resIndex, 1)
            this.$refs.pageNav.changePage(0, this.page.expandFileList[0])
          }
        }
      }
    },
    deleteFile () {
      this.$refs.folder.remove(this.currentData)
      const resIndex = this.page.expandFileList.findIndex(
        (i) => i.id === this.currentData.id
      )
      if (resIndex >= 0) {
        this.changeExpandFileList('del', null, resIndex)
      }
    },
    async deleteServer (currentData) {
      const deleteText = `确定要删除 ${currentData.service_name} 这个服务吗？`
      this.$confirm(`${deleteText}`, '确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        this.page.expandFileList = []
        deleteServiceTemplateAPI(currentData.service_name, 'helm', this.projectName, 'private').then(res => {
          if (res) {
            this.updateEnv = true
            this.$store.dispatch('queryService', { projectName: this.projectName })
          }
        }).catch(error => console.log(error))
      })
    },
    newFile (type) {
      if (this.currentData && this.currentData.type === 'file') {
        return
      }
      const id = Date.now()
      let newNode = null
      if (type === 'file') {
        newNode = cloneDeep(newFileNode)
        newNode = { id: id, ...newNode }
      } else {
        newNode = cloneDeep(newFolderNode)
        newNode = { id: id, ...newNode }
      }
      if (this.currentData) {
        if (this.currentData.children) {
          this.currentData.children.push(newNode)
        } else {
          this.currentData.children = [newNode]
        }
        this.expandKey.push(id)
      } else {
        this.nodeData.push(newNode)
      }
      this.currentData = newNode
      setTimeout(() => {
        this.$refs.folder.onFocus()
      })
    },
    updateFileName () {
      this.updateData('currentData', 'updateFileName', true)
      this.$nextTick(() => {
        this.$refs.folder.onFocus()
      })
    },
    saveNewFile (value) {
      if (value) {
        this.updateData('currentData', 'label', value)
        this.updateData('currentData', 'add', false)
        this.changeExpandFileList('add', this.currentData)
      } else {
        this.deleteFile()
      }
    },
    saveNewFolder (value) {
      if (value) {
        this.updateData('currentData', 'label', value)
        this.updateData('currentData', 'add', false)
      } else {
        this.deleteFile()
      }
    },
    saveFile () {
      const newCode = this.currentCode.cacheTxt
      if (this.currentCode.isUpdate) {
        this.updateData('currentCode', 'isUpdate', false)
        this.updateData('currentCode', 'txt', newCode)
        this.updateData('currentCode', 'cacheTxt', '')
        this.updateData('currentCode', 'updated', true)
        this.commitCache.push(this.currentCode)
      }
    },
    clearCommitCache () {
      this.commitCache = []
    },
    commit () {
      const params = {
        projectName: this.projectName,
        commitCache: this.commitCache
      }
      this.$store.dispatch('updateHelmChart', params).then(res => {
        this.updateEnv = true
        this.clearCommitCache()
      })
    },
    saveFileName (value) {
      if (value) {
        this.updateData('currentData', 'label', value)
        this.updateData('currentData', 'updateFileName', false)
      } else {
        this.updateData('currentData', 'updateFileName', false)
      }
    },
    changeModalStatus (position, status, currentData, isClose) {
      if (position) {
        this.position = position
      }
      if (status) {
        this.zindex = 998
      } else {
        this.zindex = 0
      }
      if (!isClose) {
        this.setData('currentData', currentData)
      }
      this.modalvalue = status
    },
    openRepoModal (currentService) {
      if (currentService) {
        this.currentService = currentService
      } else {
        this.currentService = null
      }
      this.dialogVisible = true
    },
    update () {
      this.updateHelmEnvDialogVisible = true
    }
  },
  watch: {
    service (value) {
      this.nodeData = cloneDeep(value)
      if (value.length === 0) {
        // const item = {
        //   id: '导入项目',
        //   type: 'components',
        //   componentsName: 'Repo',
        //   label: '导入项目',
        //   name: '导入项目',
        // }
        // this.changeExpandFileList('add', item)
      } else {
        this.$refs.folder.addExpandFileList(this.nodeData[0].children[0])
      }
    }
  },
  computed: {
    projectName () {
      return this.$route.params.project_name
    },
    ...mapState({
      service: (state) => state.service_manage.serviceList
    })
  }
}
</script>
<style lang="less" scoped>
.content {
  display: flex;
  width: 100%;
  height: 100%;
  overflow: auto;
  background-color: #fff;
  user-select: none;

  .modalContent {
    position: fixed;
    top: 0;
    right: 0;
    bottom: 0;
    left: 0;

    .modal {
      position: absolute;
      z-index: 999;
      width: 140px;
      padding-top: 10px;
      padding-bottom: 10px;
      background-color: rgb(241, 241, 241);
      border: 1px solid rgb(197, 197, 197);
      border-radius: 10px;
      box-shadow: 0 0 5px rgb(197, 197, 197);
      cursor: pointer;

      .item {
        height: 25px;
        padding-left: 20px;
        color: #606266;
        font-size: 14px;
        line-height: 25px;
        border-bottom: 1px solid #eaeefb;
      }

      .item:hover {
        color: #fff;
        background-color: rgb(55, 134, 240);
      }
    }
  }

  .left {
    display: flex;
    flex-direction: column;
    width: 230px;
    min-width: 200px;
    max-width: 400px;
    height: 100%;
    overflow: hidden;
    background-color: #fff;
    border-right: 1px solid #ebedef;

    .title {
      flex: 0 0 auto;
      height: 50px;
      padding-right: 5px;
      line-height: 50px;
      text-align: right;
      border-bottom: 1px solid rgb(230, 230, 230);

      i {
        font-size: 25px;
      }
    }

    .folder {
      flex: 1 1 auto;
      margin-top: 10px;
    }

    .bottom {
      flex: 0 0 auto;
      height: 50px;
      padding-left: 20px;
    }
  }

  .resizer1 {
    z-index: 10;
    cursor: col-resize;
  }

  .resizer1::before {
    position: absolute;
    top: 50%;
    left: 50%;
    display: block;
    width: 8px;
    height: 55px;
    margin-top: -20px;
    margin-left: -5.5px;
    background-color: #fff;
    border: 1px solid #dbdbdb;
    border-radius: 5px;
    content: "";
  }

  .resizer2 {
    z-index: 10;
    cursor: col-resize;
  }

  .resizer2::before {
    position: absolute;
    top: 50%;
    left: 50%;
    display: block;
    width: 8px;
    height: 55px;
    margin-top: -20px;
    margin-left: -5.5px;
    background-color: #fff;
    border: 1px solid #dbdbdb;
    border-radius: 5px;
    content: "";
  }

  .center {
    width: 500px;
    height: 100%;

    .header {
      position: absolute;
      top: 0;
      z-index: 99;
      width: 100%;
      height: 40px;
      overflow-x: scroll;
    }

    .code {
      box-sizing: border-box;
      height: calc(~"100% - 40px");
      margin-top: 40px;
      overflow-y: scroll;
      background-color: #fff;
    }
  }

  .center::-webkit-scrollbar {
    width: 0 !important;
  }

  .right {
    width: 100px;
    background-color: #fff;
  }
}

.custom-resizer {
  width: 100%;
}

::-webkit-scrollbar {
  display: none;
}
</style>
