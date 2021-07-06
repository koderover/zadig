<template>
  <el-dialog title="更新环境变量"
             :visible.sync="updateHelmEnvVarDialogVisible"
             width="80%">
    <div v-loading="getHelmEnvVarLoading"
         class="kv-container">
      <el-collapse v-model="activeHelmServiceNames">
        <el-collapse-item v-for="(item, index) in helmServiceYamls"
                          :key="index"
                          :title="item.service_name"
                          :name="index">
          <editor v-model="item.values_yaml"
                  :options="yamlEditorOption"
                  lang="yaml"
                  theme="xcode"
                  width="100%"
                  height="300"></editor>
        </el-collapse-item>
      </el-collapse>
    </div>
    <span slot="footer"
          class="dialog-footer">
      <el-button size="small"
                 type="primary"
                 :loading="updataHelmEnvVarLoading"
                 @click="updateHelmEnvVar">更新</el-button>
      <el-button size="small"
                 @click="cancelUpdateHelmEnvVar">取 消</el-button>
    </span>
  </el-dialog>
</template>
<script>
import { getHelmEnvVarAPI, updateHelmEnvVarAPI } from '@/api'
import editor from 'vue2-ace-bind'
import 'brace/mode/yaml'
import 'brace/theme/xcode'
import 'brace/ext/searchbox'
export default {
  name: 'updateHelmVarDialog',
  props: {
    productInfo: Object,
    fetchAllData: Function,
    projectName: String,
    envName: String
  },
  components: {
    editor
  },
  data () {
    return {
      updateHelmEnvVarDialogVisible: false,
      helmServiceYamls: [],
      updataHelmEnvVarLoading: false,
      activeHelmServiceNames: [],
      yamlEditorOption: {
        enableEmmet: true,
        showLineNumbers: false,
        showGutter: false,
        showPrintMargin: false,
        tabSize: 2
      },
      getHelmEnvVarLoading: true
    }
  },
  methods: {
    openDialog () {
      this.updateHelmEnvVarDialogVisible = true
    },
    async getHelmEnvVar () {
      this.getHelmEnvVarLoading = true
      const projectName = this.projectName
      const envName = this.envName
      const res = await getHelmEnvVarAPI(projectName, envName).catch((error) => {
        console.log(error)
        this.getHelmEnvVarLoading = false
      })
      this.getHelmEnvVarLoading = false
      if (res) {
        this.helmServiceYamls = res
      }
    },
    updateHelmEnvVar () {
      const projectName = this.productInfo.product_name
      const envName = this.productInfo.env_name
      const payload = {
        chart_infos: this.$utils.cloneObj(this.helmServiceYamls)
      }
      this.updataHelmEnvVarLoading = true
      updateHelmEnvVarAPI(projectName, envName, payload).then(response => {
        this.updataHelmEnvVarLoading = false
        this.updateHelmEnvVarDialogVisible = false
        this.helmServiceYamls = []
        this.fetchAllData()
        this.$message({
          message: '更新环境变量成功，请等待服务升级',
          type: 'success'
        })
      }).catch(() => {
        this.updataHelmEnvVarLoading = false
      })
    },
    cancelUpdateHelmEnvVar () {
      this.updateHelmEnvVarDialogVisible = false
      this.helmServiceYamls = []
    }
  },
  watch: {
    updateHelmEnvVarDialogVisible (value) {
      if (value) {
        this.getHelmEnvVar()
      }
    }
  }
}
</script>
