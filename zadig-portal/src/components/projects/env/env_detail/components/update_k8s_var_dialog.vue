<template>
  <el-dialog
    title="更新环境变量"
    :visible.sync="updateK8sEnvVarDialogVisible"
    width="40%"
  >
    <span
      >全局变量列表
      <el-tooltip
        effect="dark"
        content="环境所用的变量，你可以修改原有 Value，也可以保持原样"
        placement="top"
      >
        <span class="el-icon-question"></span>
      </el-tooltip>
    </span>
    <div class="kv-container">
      <el-table :data="vars" style="width: 100%">
        <el-table-column label="Key">
          <template slot-scope="scope">
            <span>{{ scope.row.key }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Value">
          <template slot-scope="scope">
            <el-input
              size="small"
              v-model="scope.row.value"
              type="textarea"
              :autosize="{ minRows: 1, maxRows: 4 }"
              placeholder="请输入 Value"
            ></el-input>
          </template>
        </el-table-column>
        <el-table-column label="关联服务">
          <template slot-scope="scope">
            <span>{{
              scope.row.services ? scope.row.services.join(',') : '-'
            }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态">
          <template slot-scope="scope">
            <span>{{ scope.row.state }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="50">
          <template slot-scope="scope">
            <template>
              <span class="operate">
                <el-tooltip
                  v-if="scope.row.state === 'present'"
                  effect="dark"
                  content="模板中用到的渲染 Key 无法被删除"
                  placement="top"
                >
                  <span class="el-icon-question"></span>
                </el-tooltip>
                <el-button
                  v-else
                  type="text"
                  @click="deleteRenderKey(scope.$index, scope.row.state)"
                  class="delete"
                  >移除</el-button
                >
              </span>
            </template>
          </template>
        </el-table-column>
      </el-table>
    </div>
    <span slot="footer" class="dialog-footer">
      <el-button
        size="small"
        type="primary"
        :loading="updataK8sEnvVarLoading"
        @click="updateK8sEnvVar"
        >更新</el-button
      >
      <el-button size="small" @click="updateK8sEnvVarDialogVisible = false"
        >取 消</el-button
      >
    </span>
  </el-dialog>
</template>
<script>
import { updateK8sEnvAPI } from '@/api'
import { cloneDeep } from 'lodash'
export default {
  name: 'updateK8sVar',
  props: {
    fetchAllData: Function,
    productInfo: Object,
  },
  data() {
    return {
      updateK8sEnvVarDialogVisible: false,
      updataK8sEnvVarLoading: false,
      vars: [],
    }
  },
  methods: {
    openDialog() {
      this.updateK8sEnvVarDialogVisible = true
    },
    updateK8sEnvVar() {
      const projectName = this.productInfo.product_name
      const envName = this.productInfo.env_name
      const envType = this.productInfo.isProd ? 'prod' : ''
      const payload = { vars: this.vars }
      this.updataK8sEnvVarLoading = true
      updateK8sEnvAPI(projectName, envName, payload, envType).then(
        (response) => {
          this.updataK8sEnvVarLoading = false
          this.updateK8sEnvVarDialogVisible = false
          this.fetchAllData()
          this.$message({
            message: '更新环境变量成功，请等待服务升级',
            type: 'success',
          })
        }
      )
    },
    deleteRenderKey(index, state) {
      this.vars.splice(index, 1)
    },
  },
  watch: {
    updateK8sEnvVarDialogVisible(value) {
      if (value) {
        this.vars = cloneDeep(this.productInfo.vars)
      }
    },
  },
}
</script>