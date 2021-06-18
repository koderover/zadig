<template>
  <div v-loading="loading"
       element-loading-text="正在获取配置"
       element-loading-spinner="el-icon-loading"
       class="config-overview-container">
    <el-dialog :fullscreen="true"
               :visible.sync="dialogEditorVisible"
               v-loading="settingLoading"
               element-loading-spinner="el-icon-loading"
               custom-class="editor-dialog"
               :show-close=false>
      <el-row class="operation">
        <el-button type="primary"
                   :loading="checkUpdateFlag"
                   @click="handleConfigEdit('confirm',fullScreenEditObj['configName'],fullScreenEditObj['configIndex'])"
                   plain>保存</el-button>
        <el-button type="info"
                   plain
                   @click="cancelSave">取消</el-button>
      </el-row>
      <div class="show-diff-button">
        <el-button v-if="showUpdateButton"
                   type="primary"
                   @click="updateValue=!updateValue"
                   :icon="updateValue?'el-icon-arrow-right':'el-icon-arrow-left'"
                   size="mini"
                   plain
                   circle></el-button>
      </div>
      <editor v-model="fullScreenEditObj.value"
              lang="sh"
              theme="xcode"
              :width="editorProperty.width"
              :height="editorProperty.height"
              :options="option"></editor>
      <div class="show-diff"
           v-if="updateValue">
        <h1>修改期间后台配置变动</h1>
        <hr />
        <div class="diff-now">
          <pre><!--
       --><div v-for="(section, index) in configDiff" :key="index"
           :class="{ 'added': section.added, 'removed': section.removed }"><!--
         --><span v-if="section.added" class="code-line-prefix"> + </span><!--
         --><span v-if="section.removed" class="code-line-prefix"> - </span><!--
         --><span >{{ section.value }}</span><!--
       --></div><!--
     --></pre>
        </div>
      </div>
    </el-dialog>
    <div v-if="configMaps.length === 0"
         class="no-config">
      <h3>暂无配置</h3>
    </div>
    <div v-else
         v-for="(config,index) in configMaps"
         :key="index"
         class="config-container">
      <div class="type">
        <h3>{{ config.current.name }}</h3>
        <p class="tip">注意：修改服务配置会重启服务</p>
        <el-button @click="showHistory(config)"
                   type="primary"
                   plain
                   size="mini"
                   icon="ion-android-list">历史配置</el-button>
      </div>
      <el-table :data="config.current.data"
                style="width: 100%;">
        <el-table-column label="Key">
          <template slot-scope="scope">
            <span>{{ scope.row.key }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Value">
          <template slot-scope="scope">
            <span v-if="!editConfigValueVisable[config.current.name][scope.row.key]">
              {{ $utils.tailCut(scope.row.value,80)}}</span>
            <el-input v-else
                      size="small"
                      v-model="scope.row.value"
                      type="textarea"
                      :autosize="{ minRows: 2, maxRows: 4}"
                      placeholder="请输入 value"></el-input>
          </template>
        </el-table-column>
        <el-table-column width="100">
          <template slot-scope="scope">
            <el-button v-if="!editConfigValueVisable[config.current.name][scope.row.key]"
                       @click="fullScreenEdit(index,scope.$index,config.current.name,scope.row.key,scope.row.value,'edit')"
                       size="mini"
                       icon="el-icon-edit">修改</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog title="配置diff"
               :visible.sync="historyVisible"
               width="70%"
               class="config-history-dialog">
      <div>
        <el-button @click="showDiff"
                   type="primary"
                   plain
                   size="mini"
                   icon="ion-eye">比较所选版本</el-button>
      </div>

      <el-table :data="histories"
                @selection-change="selectionChanged"
                ref="configHistoryTable">
        <el-table-column type="selection"></el-table-column>
        <el-table-column prop="name"
                         label="版本">
          <template slot-scope="scope">
            <span >{{scope.row.version}}</span>
          </template>
        </el-table-column>
        <el-table-column prop="creationTimestamp"
                         label="创建时间">
          <template slot-scope="scope">
            <span>{{moment(scope.row.creationTimestamp).format('YYYY-MM-DD HH:mm')}}</span>
          </template>
        </el-table-column>
        <el-table-column prop="modifiedBy"
                         label="最后修改"></el-table-column>
        <el-table-column label="操作"
                         width="">
          <template slot-scope="scope">
            <el-button v-if="scope.$index!==0"
                       @click="rollbackTo(scope.row)"
                       icon="el-icon-refresh-left"
                       size="mini">回滚</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <el-dialog :title="diffTitle"
               :visible.sync="diffVisible"
               width="60%"
               class="log-diff-container">
      <div class="diff-content">
        <pre><!--
       --><div v-for="(section, index) in configDiff" :key="index"
           :class="{ 'added': section.added, 'removed': section.removed }"><!--
         --><span v-if="section.added" class="code-line-prefix"> + </span><!--
         --><span v-if="section.removed" class="code-line-prefix"> - </span><!--
         --><span >{{ section.value }}</span><!--
       --></div><!--
     --></pre>
      </div>
    </el-dialog>

  </div>
</template>

<script>
import aceEditor from 'vue2-ace-bind'
import 'brace/mode/javascript'
import 'brace/mode/sh'
import 'brace/theme/chrome'
import 'brace/theme/xcode'
import 'brace/theme/terminal'
import 'brace/ext/searchbox'
import moment from 'moment'
import qs from 'qs'
import bus from '@utils/event_bus'
import { getConfigmapAPI, updateConfigmapAPI, rollbackConfigmapAPI } from '@api'
import _ from 'lodash'
const jsdiff = require('diff')

export default {
  data () {
    return {
      moment,
      srcConfigName: null,
      window: window,
      editConfigValueVisable: {},
      configMaps: [],
      fullScreenEditObj: {
        configName: '',
        value: '',
        key: '',
        configIndex: 0,
        keyIndex: 0
      },
      dialogEditorVisible: false,
      option: {
        enableEmmet: true,
        showLineNumbers: false,
        showFoldWidgets: true,
        showGutter: false,
        displayIndentGuides: false,
        showPrintMargin: false
      },
      editorProperty: {
        width: '100%',
        height: this.$utils.getViewPort().height - 60
      },
      historyVisible: false,
      histories: [],
      selectedHistories: [],
      loading: false,
      diffVisible: false,
      configDiff: [],
      updateValue: false,
      showUpdateButton: false,
      checkUpdateFlag: false,
      settingLoading: false
    }
  },
  computed: {
    projectName () {
      return this.$route.params.project_name
    },
    serviceName () {
      return this.$route.params.service_name
    },
    envName () {
      return this.$route.query.envName || ''
    },
    isProd () {
      return this.$route.query.isProd === 'true'
    },
    envType () {
      return this.isProd ? 'prod' : ''
    },
    orderedHistoriesDesc () {
      if (Array.isArray(this.selectedHistories) && this.selectedHistories.length > 1) {
        const arr = Array.from(this.selectedHistories)
        if (arr[1]._idx < arr[0]._idx) {
          arr.reverse()
        }
        return arr
      }
      return this.selectedHistories
    },
    diffTitle () {
      const candidates = this.orderedHistoriesDesc
      if (Array.isArray(candidates) && candidates.length > 1) {
        return `${candidates[0].version} 相对于 ${candidates[1].version} 的 diff`
      }
      return '配置 diff（未勾选）'
    }
  },
  methods: {
    cancelSave () {
      this.dialogEditorVisible = false
      this.updateValue = false
      this.showUpdateButton = false
      this.configDiff = []
    },
    async getConfigmap () {
      const query = {
        productName: this.projectName,
        serviceName: this.serviceName,
        envType: this.envType
      }
      this.envName && (query.envName = this.envName)
      this.loading = true
      await getConfigmapAPI(qs.stringify(query)).then(res => {
        this.loading = false
        this.configMaps = res.sort(function (a, b) {
          if (a.current.name < b.current.name) {
            return -1
          }
          if (a.current.name > b.current.name) {
            return 1
          }
          return 0
        })
        this.convertMapToArr(this.configMaps)
      })
    },
    async fullScreenEdit (config_index, key_index, config_name, key, value, operation) {
      if (operation === 'edit') {
        this.dialogEditorVisible = true
        this.fullScreenEditObj.configName = config_name
        this.fullScreenEditObj.key = key
        this.fullScreenEditObj.configIndex = config_index
        this.fullScreenEditObj.keyIndex = key_index
        this.settingLoading = true
        await this.getConfigmap()
        if (!(value === this.configMaps[config_index].current.data[key_index].value)) {
          this.fullScreenEditObj.value = this.configMaps[config_index].current.data[key_index].value
        } else {
          this.fullScreenEditObj.value = value
        }
        this.settingLoading = false
      }
    },
    async checkUpdate (config_index, key_index, value) {
      this.checkUpdateFlag = true
      await this.getConfigmap()
      this.checkUpdateFlag = false
      if (!(value === this.configMaps[config_index].current.data[key_index].value)) {
        this.$message({
          message: '环境中的服务配置有更新，请确认配置变动后再继续保存！',
          type: 'warning'
        })
        this.updateValue = true
        this.showUpdateButton = true
        this.configDiff = jsdiff.diffLines(
          value.replace(/\\n/g, '\n').replace(/\\t/g, '\t'),
          this.configMaps[config_index].current.data[key_index].value.replace(/\\n/g, '\n').replace(/\\t/g, '\t')
        )
        return true
      } else {
        this.updateValue = false
        this.showUpdateButton = false
        this.configDiff = []
        return false
      }
    },
    async handleConfigEdit (operation, config_name, key) {
      if (operation === 'confirm') {
        const _config_index = this.fullScreenEditObj.configIndex
        const _key_index = this.fullScreenEditObj.keyIndex
        const _value = this.fullScreenEditObj.value
        if (await this.checkUpdate(_config_index, _key_index, this.configMaps[_config_index].current.data[_key_index].value)) {
          return
        }
        this.configMaps[_config_index].current.data[_key_index].value = _value
        this.saveCurrentEditConfig(config_name, 'update')
        this.dialogEditorVisible = false
      }
    },
    saveCurrentEditConfig (config_name, operation) {
      const configName = config_name
      const envType = this.envType
      const payload = {
        env_name: this.envName,
        product_name: this.projectName,
        service_name: this.serviceName,
        config_name: configName,
        data: this.findConfigAndConvert(configName)
      }
      if (operation === 'update') {
        updateConfigmapAPI(envType, payload).then(res => {
          this.getConfigmap()
          this.$message({
            message: '配置保存成功',
            type: 'success'
          })
        })
      }
    },
    convertMapToArr (configmap) {
      const buildMap = obj => {
        const arrPair = []
        const map = new Map()
        Object.keys(obj).forEach(key => {
          map.set(key, obj[key])
        })
        for (const [_key, _value] of map) {
          arrPair.push({
            key: _key,
            value: _value
          })
        }
        return arrPair
      }
      const newPair = configmap.map(config => {
        const currentConfig = config.current
        this.$set(this.editConfigValueVisable, currentConfig.name, {})
        if (currentConfig.data === null) {
          currentConfig.data = {
            暂无配置: '暂无配置'
          }
        }
        currentConfig.data = buildMap(currentConfig.data)
        currentConfig.data.forEach(element => {
          this.$set(this.editConfigValueVisable[currentConfig.name], [element.key], false)
        })
        return currentConfig
      })
      return newPair
    },
    findConfigAndConvert (config_name) {
      let arr = {}
      this.configMaps.forEach(config => {
        if (config.current.name === config_name) {
          arr = config.current.data
        }
      })
      const result = arr.reduce(function (map, obj) {
        map[obj.key] = obj.value
        return map
      }, {})
      return result
    },

    showHistory (config) {
      this.historyVisible = true
      this.histories = _.cloneDeep(config.historicalRevisions)
      this.histories.unshift(config.current)
      this.adaptHistories()
      this.srcConfigName = config.current.name
    },

    selectionChanged (val) {
      if (val.length > 2) {
        this.$message({
          message: '只能选择两个版本用于比较',
          type: 'warning'
        })
        this.$refs.configHistoryTable.toggleRowSelection(val[val.length - 1])
        return false
      }

      this.selectedHistories = val
    },

    showDiff () {
      const candidates = this.selectedHistories
      if (candidates.length !== 2) {
        this.$message({
          message: '只能选择两个版本用于比较',
          type: 'warning'
        })
        return
      }

      this.diffVisible = true
      this.configDiff = jsdiff.diffLines(
        JSON.stringify(this.$utils.cloneObj(this.orderedHistoriesDesc[1].data), null, 2).replace(/\\n/g, '\n').replace(/\\t/g, '\t'),
        JSON.stringify(this.$utils.cloneObj(this.orderedHistoriesDesc[0].data), null, 2).replace(/\\n/g, '\n').replace(/\\t/g, '\t')
      )
    },
    adaptHistories () {
      const len = this.histories.length
      this.histories.forEach((hist, i) => {
        hist.version = i === 0 ? '当前' : `历史${len - i}`
        hist._idx = i
      })
      return this.histories
    },
    rollbackTo (dest) {
      this.$confirm(`确定要回滚到 ${dest.version} 吗`, '确认回滚', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        const payload = {
          env_name: this.envName,
          product_name: this.projectName,
          service_name: this.serviceName,
          src_config_name: dest.name,
          destin_config_name: this.srcConfigName
        }
        const envType = this.envType
        this.envName && (payload.env_name = this.envName)
        rollbackConfigmapAPI(envType, payload).then(res => {
          this.$message.success({
            message: '配置回滚成功，正在重启服务',
            type: 'success'
          })
          this.historyVisible = false
          this.diffVisible = false
          this.getConfigmap()
        })
      })
    }
  },
  created () {
    this.getConfigmap()
    bus.$emit(`set-topbar-title`,
      {
        title: '',
        breadcrumb: [
          { title: '项目', url: '/v1/projects' },
          { title: this.projectName, url: `/v1/projects/detail/${this.projectName}` },
          { title: '集成环境', url: `/v1/projects/detail/${this.projectName}/envs/detail?envName=${this.envName}` },
          { title: this.envName, url: `/v1/projects/detail/${this.projectName}/envs/detail?envName=${this.envName}` },
          { title: this.serviceName, url: `/v1/projects/detail/${this.projectName}/envs/detail/${this.serviceName}${window.location.search}` },
          { title: '配置管理', url: `` }
        ]
      })
  },
  components: {
    editor: aceEditor
  }
}
</script>

<style lang="less">
.log-diff-container {
  .diff-content {
    height: 600px;
    overflow-y: auto;

    .added {
      background-color: #b4e2b4;
    }

    .removed {
      background-color: #ffb6ba;
    }
  }
}

.config-overview-container {
  position: relative;
  flex: 1;
  padding: 10px 20px;
  overflow: auto;
  font-size: 15px;

  .module-title h1 {
    margin-bottom: 1.5rem;
    font-weight: 200;
    font-size: 2.4rem;
  }

  .no-config {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 20px;

    h3 {
      color: #ccc;
    }
  }

  .editor-dialog {
    position: fixed;

    .show-diff-button {
      position: absolute;
      top: 70px;
      right: 30px;
      z-index: 2;
    }

    .show-diff {
      position: absolute;
      top: 60px;
      right: 20px;
      z-index: 1;
      box-sizing: border-box;
      width: 50%;
      height: 40%;
      padding: 10px;
      background: #fff;
      border-radius: 3px;
      box-shadow: 0 0 6px 3px #ddd;

      h1 {
        margin: 0;
        font-weight: 600;
        font-size: 1.2rem;
        line-height: 1.5;
      }

      hr {
        color: #eee;
      }

      .diff-now {
        height: 85%;
        margin: 0 10px;
        overflow: auto;

        .added {
          background-color: #b4e2b4;
        }

        .removed {
          background-color: #ffb6ba;
        }
      }

      .el-button {
        position: absolute;
        right: 10px;
        bottom: 10px;
      }
    }

    .el-dialog__headerbtn {
      top: 6px;
      font-size: 30px;
    }

    .el-dialog__header {
      padding: 0;
    }

    .el-dialog__body {
      padding: 10px 0;

      .operation {
        margin-right: 20px;
        text-align: right;
      }
    }
  }

  .config-history-dialog {
    .el-table-column--selection.is-leaf > .cell {
      display: none;
    }
  }
}

.config-container {
  margin-bottom: 35px;

  .type {
    margin-bottom: 1rem;

    h3 {
      display: inline-block;
      color: #000;
      font-size: 18px;
      border-bottom: 1px solid transparent;

      &:hover {
        /* border-bottom-color: #5e6166; */
      }
    }

    .tip {
      color: #e6a23c;
    }
  }

  .edit,
  .confirm {
    margin-left: 2px;
    color: #8d9199;
    font-size: 1.2em;
    cursor: pointer;

    &:hover {
      color: #1989fa;
    }
  }

  .cancel {
    margin-left: 2px;
    color: #8d9199;
    font-size: 1.2em;
    cursor: pointer;

    &:hover {
      color: #ff4949;
    }
  }

  .add-env-container {
    margin-top: -1px;
  }

  .add-env-btn {
    display: inline-block;
    margin-top: 10px;
    margin-left: 5px;

    i {
      padding-right: 4px;
      color: #5e6166;
      font-size: 16px;
      line-height: 14px;
      cursor: pointer;

      &:hover {
        color: #1989fa;
      }
    }
  }
}
</style>
