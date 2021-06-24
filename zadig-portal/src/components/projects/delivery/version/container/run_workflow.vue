<template>
  <el-form class="run-workflow"
           label-width="90px">
    <el-form-item prop="productName"
                  label="集成环境">
      <el-select v-model="runner.envAndNamespace"
                 size="medium"
                 :disabled="specificEnv"
                 class="full-width">
        <el-option v-for="pro of matchedProducts"
                   :key="`${pro.product_name}/${pro.env_name}`"
                   :label="`${pro.product_name}/${pro.env_name}${pro.is_prod?'（生产）':''}`"
                   :value="`${pro.product_name}/${pro.env_name}`">
          <span>{{`${pro.product_name} / ${pro.env_name}`}}
            <el-tag v-if="pro.is_prod"
                    type="danger"
                    size="mini"
                    effect="light">
              生产
            </el-tag>
          </span>
        </el-option>
        <el-option v-if="matchedProducts.length===0"
                   label=""
                   value="">
          <router-link style="color: #909399;"
                       :to="`/v1/projects/detail/${projectName}/envs/create`">
            {{`(环境不存在或者没有权限，点击创建环境)`}}
          </router-link>
        </el-option>
      </el-select>
      <el-tooltip v-if="specificEnv"
                  effect="dark"
                  content="该工作流已指定环境运行，可通过修改 工作流->基本信息 来解除指定环境绑定"
                  placement="top">
        <span><i style="color: #909399;"
             class="el-icon-question"></i></span>
      </el-tooltip>
    </el-form-item>
    <el-table v-if="imageConfigs.length > 0"
              :data="imageConfigs"
              empty-text="无"
              class="service-deploy-table">
      <el-table-column prop="serviceName"
                       label="服务"
                       width="150px">
      </el-table-column>
      <el-table-column label="镜像名称">
        <template slot-scope="scope">
          <div class="workflow-build-rows">
            <el-row class="build-row">
              <template>
                {{scope.row.registryName}}
              </template>
            </el-row>
          </div>
        </template>
      </el-table-column>
    </el-table>
    <div class="start-task">
      <el-button @click="submit"
                 :loading="startTaskLoading"
                 type="primary"
                 size="small">
        {{ startTaskLoading?'启动中':'启动任务' }}
      </el-button>

    </div>

  </el-form>
</template>

<script>
import { listProductAPI, runWorkflowAPI } from '@api'

export default {
  data () {
    return {
      matchedProducts: [],
      specificEnv: false,
      startTaskLoading: false,
      runner: {
        envAndNamespace: '',
        workflow_name: '',
        product_tmpl_name: '',
        namespace: '',
        artifact_args: [],
        registry_id: ''
      }
    }
  },
  computed: {
    artifactDeployEnabled () {
      return this.workflowMeta.artifact_stage && this.workflowMeta.artifact_stage.enabled
    }
  },
  methods: {
    async filterProducts (products) {
      const prodProducts = products.filter(element => {
        if (element.product_name === this.projectName) {
          if (element.is_prod) {
            return element
          }
        }
        return false
      })
      const testProducts = products.filter(element => {
        if (element.product_name === this.projectName) {
          if (!element.is_prod) {
            return element
          }
        }
        return false
      })
      this.matchedProducts = prodProducts.concat(testProducts)
    },
    submit () {
      const projectName = this.projectName
      const artifactDeployEnabled = true
      if (!this.checkInput()) {
        return
      }
      this.startTaskLoading = true
      this.runner.product_tmpl_name = this.runner.envAndNamespace.trim().split('/')[0]
      this.runner.namespace = this.runner.envAndNamespace.trim().split('/')[1]
      this.runner.artifact_args = this.imageConfigs.map(element => {
        return {
          name: element.serviceName.split('/')[1],
          service_name: element.serviceName.split('/')[0],
          image: element.registryName.split('/')[2],
          deploy: [{
            env: `${element.serviceName}`,
            type: 'k8s',
            product_name: this.runner.product_tmpl_name
          }]
        }
      })
      this.runner.registry_id = this.imageConfigs[0].registryId
      const clone = this.$utils.cloneObj(this.runner)
      runWorkflowAPI(clone, artifactDeployEnabled).then(res => {
        const taskId = res.task_id
        const pipelineName = res.pipeline_name
        this.$message.success('创建成功')
        this.$emit('success')
        this.$router.push(`/v1/projects/detail/${projectName}/pipelines/multi/${pipelineName}/${taskId}?status=running`)
        this.$store.dispatch('refreshWorkflowList')
      }).catch(error => {
        console.log(error)
        if (error.response && error.response.data.code === 6168) {
          const projectName = error.response.data.extra.productName
          const envName = error.response.data.extra.envName
          const serviceName = error.response.data.extra.serviceName
          this.$message({
            message: `检测到 ${projectName} 中 ${envName} 环境下的 ${serviceName} 服务未启动 <br> 请检查后再运行工作流`,
            type: 'warning',
            dangerouslyUseHTMLString: true,
            duration: 5000
          })
          this.$router.push(`/v1/projects/detail/${projectName}/envs/detail/${serviceName}?envName=${envName}&projectName=${projectName}`)
        }
      }).finally(() => {
        this.startTaskLoading = false
      })
    },
    checkInput () {
      if (!this.runner.envAndNamespace) {
        this.$message.error('请选择集成环境')
        return false
      } else {
        return true
      }
    }
  },
  created () {
    const projectName = this.projectName
    this.runner.workflow_name = this.workflowName
    listProductAPI('', projectName).then(res => {
      this.filterProducts(res)
    })
  },
  props: {
    workflowName: {
      type: String,
      required: true
    },
    projectName: {
      type: String,
      required: true
    },
    imageConfigs: {
      type: Array,
      required: true
    }
  },
  components: {
  }
}
</script>

<style lang="less">
.run-workflow {
  .service-deploy-table,
  .test-table {
    width: auto;
    margin-bottom: 15px;
    padding: 0 5px;
  }

  .advanced-setting {
    margin-bottom: 10px;
    padding: 0 0;
  }

  .el-input,
  .el-select {
    &.full-width {
      width: 40%;
    }
  }

  .start-task {
    margin-bottom: 10px;
    margin-left: 10px;
  }
}
</style>
