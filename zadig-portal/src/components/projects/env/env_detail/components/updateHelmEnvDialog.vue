<template>
  <div>
    <el-dialog
      title="更新环境"
      :visible.sync="updateHelmEnvDialogVisible"
      width="40%"
    >
        <el-table
          :data="envChartsDiff"
          v-loading="getHelmEnvChartDiffLoading"
          element-loading-text="正在获取版本信息"
          style="width: 100%;"
        >
          <el-table-column label="Chart 名称">
            <template slot-scope="scope">
              <span>{{ scope.row.service_name }}</span>
            </template>
          </el-table-column>
          <el-table-column label="当前版本">
            <template slot="header">
              <span
                >当前版本
                <el-tooltip
                  effect="dark"
                  content="集成环境中目前使用的 Chart 版本"
                  placement="top"
                  ><i class="el-icon-question"></i>
                </el-tooltip>
              </span>
            </template>
            <template slot-scope="scope">
              <span>{{ scope.row.current_version }}</span>
            </template>
          </el-table-column>
          <el-table-column label="新版本">
            <template slot="header">
              <span
                >新版本
                <el-tooltip
                  effect="dark"
                  content="服务配置中的 Chart 版本"
                  placement="top"
                  ><i class="el-icon-question"></i>
                </el-tooltip>
              </span>
            </template>
            <template slot-scope="scope">
              <span>{{ scope.row.latest_version }}</span>
            </template>
          </el-table-column>
          <el-table-column label="操作">
            <template slot-scope="scope">
              <el-button
                size="mini"
                type="primary"
                @click="helmChartDiff(scope.row)"
                plain
                >版本对比</el-button
              >
            </template>
          </el-table-column>
        </el-table>
        <span slot="footer" class="dialog-footer">
          <el-button
            size="small"
            type="primary"
            :loading="updataHelmEnvLoading"
            @click="updateHelmEnv()"
            >确 定</el-button
          >
          <el-button size="small" @click="updateHelmEnvDialogVisible = false"
            >取 消</el-button
          >
        </span>
    </el-dialog>
    <el-dialog
      :title="helmChartDiffTitle"
      :visible.sync="helmChartDiffVisible"
      width="60%"
      class="diff-popper"
    >
      <div class="diff-container">
        <div class="diff-content">
          <pre><!--
       --><div v-for="(section, index) in helmChartDiffResult" :key="index"
           :class="{ 'added': section.added, 'removed': section.removed }"><!--
         --><span v-if="section.added" class="code-line-prefix"> + </span><!--
         --><span v-if="section.removed" class="code-line-prefix"> - </span><!--
         --><span >{{ section.value }}</span><!--
       --></div><!--
     --></pre>
        </div>
      </div>
    </el-dialog>
  </div>
</template>
<script>
import { getHelmEnvChartDiffAPI, updateHelmEnvAPI } from '@/api'
const jsdiff = require('diff')
export default {
  name: 'updateHelmEnv',
  props: {
    productInfo: Object,
    fetchAllData: Function
  },
  data () {
    return {
      updateHelmEnvDialogVisible: false,
      envChartsDiff: [],
      getHelmEnvChartDiffLoading: false,
      updataHelmEnvLoading: false,
      helmChartDiffTitle: '',
      helmChartDiffVisible: false,
      helmChartDiffResult: []
    }
  },
  methods: {
    openDialog () {
      this.updateHelmEnvDialogVisible = true
    },
    updateHelmEnv () {
      this.$confirm('更新环境, 是否继续?', '更新', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
        .then(() => {
          this.updataHelmEnvLoading = true
          const projectName = this.productInfo.product_name
          const envName = this.productInfo.env_name
          const updateType = 'envVar'
          updateHelmEnvAPI(projectName, envName, updateType).then(
            (response) => {
              this.updateHelmEnvDialogVisible = false
              this.updataHelmEnvLoading = false
              this.fetchAllData()
              this.$message({
                message: '更新环境成功，请等待服务升级',
                type: 'success'
              })
            }
          )
        })
        .catch(() => {
          this.$message({
            type: 'info',
            message: '已取消更新'
          })
        })
    },
    helmChartDiff (scope) {
      this.helmChartDiffTitle = `${scope.service_name} 版本 ${scope.current_version} 与 ${scope.latest_version} 的对比`
      this.helmChartDiffVisible = true
      this.helmChartDiffResult = jsdiff.diffLines(
        scope.current_values_yaml,
        scope.latest_values_yaml
      )
    },
    async getHelmEnvChartDiff () {
      const projectName = this.productInfo.product_name
      const envName = this.productInfo.env_name
      this.getHelmEnvChartDiffLoading = true
      const res = await getHelmEnvChartDiffAPI(
        projectName,
        envName
      ).catch((error) => console.log(error))
      if (res) {
        this.envChartsDiff = res
      }
      this.getHelmEnvChartDiffLoading = false
    }
  },
  watch: {
    updateHelmEnvDialogVisible (value) {
      if (value) {
        this.getHelmEnvChartDiff()
      }
    }
  }
}
</script>
