<template>
  <div class="function-test-manage">
    <el-dialog title="选择关联的工作流"
               :visible.sync="selectWorkflowDialogVisible"
               width="30%"
               center>
      <el-select v-model="selectWorkflow"
                 style="width: 100%;"
                 filterable
                 value-key="name"
                 size="small"
                 placeholder="请选择要关联的工作流，支持搜索">
        <el-option v-for="(workflow,index) in availableWorkflows"
                   :key="index"
                   :label="`${workflow.name} (${workflow.product_tmpl_name})`"
                   :value="workflow">
        </el-option>
      </el-select>
      <span slot="footer"
            class="dialog-footer">
        <el-button size="small"
                   @click="selectWorkflowDialogVisible = false">取 消</el-button>
        <el-button type="primary"
                   size="small"
                   :disabled="!selectWorkflow"
                   @click="bindWorkflow">确 定</el-button>
      </span>
    </el-dialog>
    <div class="tab-container">
      <el-tabs @tab-click="changeRoute"
               v-model="activeTab"
               :tab-position="tabPosition"
               type="card">
        <el-tab-pane @tab-click="changeRoute('function')"
                     name="function"
                     label="功能测试">
          <template>
            <el-alert type="info"
                      :closable="false"
                      description="功能测试，可定义自动化功能测试脚本">
            </el-alert>
          </template>
          <div class="add-test">
            <router-link :to="`/v1/${basePath}/detail/${projectName}/test/add/function`">
              <el-button size="small"
                         type="primary"
                         plain>添加</el-button>
            </router-link>
          </div>
          <function-test-list :testList="testList"
                              :jobPrefix="`/v1/${basePath}/detail/${projectName}/test/detail/function`"
                              :projectName="projectName"
                              :basePath="basePath"
                              @removeTest="removeTest"
                              @addConnection="addConnection"
                              @deleteConnection="deleteConnection"
                              @runTests="runTests"
                              inProject></function-test-list>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script>
import functionTestList from './container/function_test_list.vue'
import moment from 'moment'
import { testsAPI, runTestsAPI, updateWorkflowAPI, deleteTestAPI, getWorkflowBindAPI, singleTestAPI } from '@api'

export default {
  data () {
    return {
      testList: [],
      activeTab: 'function',
      tabPosition: 'top',
      selectWorkflowDialogVisible: false,
      currentTestName: '',
      selectWorkflow: null,
      availableWorkflows: []
    }
  },
  props: {
    basePath: {
      request: true,
      type: String
    }
  },
  computed: {
    projectName () {
      return this.$route.params.project_name
    }
  },
  methods: {
    fetchTestList () {
      const projectName = this.projectName
      const testType = 'function'
      testsAPI(projectName, testType).then(res => {
        for (const row of res) {
          row.updateTimeReadable = moment(row.update_time, 'X').format('YYYY-MM-DD HH:mm:ss')
        }
        this.testList = res
      })
    },
    async bindWorkflow () {
      const workflow = this.selectWorkflow
      if (workflow.test_stage.test_names) {
        workflow.test_stage.enabled = true
        workflow.test_stage.test_names.push(this.currentTestName)
      } else {
        const res = await singleTestAPI(this.currentTestName, this.projectName)
        workflow.test_stage.enabled = true
        workflow.test_stage.tests = workflow.test_stage.tests || []
        workflow.test_stage.tests.push({
          test_name: this.currentTestName,
          envs: res.pre_test.envs || []
        })
      }
      updateWorkflowAPI(workflow).then(() => {
        this.$message({
          type: 'success',
          message: '关联工作流成功'
        })
        this.selectWorkflow = null
        this.availableWorkflows = []
        this.selectWorkflowDialogVisible = false
        this.fetchTestList()
      })
    },
    runTests (name) {
      const payload = {
        product_name: this.projectName,
        test_name: name
      }
      runTestsAPI(payload).then((res) => {
        this.$message.success('任务启动成功')
        return res
      }).then((res) => {
        this.$router.push(`/v1/${this.basePath}/detail/${this.projectName}/test/detail/function/${res.pipeline_name}/${res.task_id}?status=running`)
      })
    },
    removeTest (obj) {
      const projectName = this.projectName
      this.$confirm(`确定要删除 ${obj.name} 吗？`, '确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        deleteTestAPI(obj.name, projectName).then(() => {
          this.$message.success('删除成功')
          this.fetchTestList()
        })
      })
    },
    addConnection (testName) {
      this.selectWorkflowDialogVisible = true
      this.currentTestName = testName
      getWorkflowBindAPI(testName).then((res) => {
        this.availableWorkflows = res.filter(element => {
          return element.product_tmpl_name
        })
      })
    },
    deleteConnection (testName, workflow) {
      this.$confirm(`确定要取消和工作流 ${workflow.name} 的关联`, '取消关联', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        if (workflow.test_stage.test_names) {
          const index = workflow.test_stage.test_names.indexOf(testName)
          workflow.test_stage.test_names.splice(index, 1)
        } else if (workflow.test_stage.tests) {
          workflow.test_stage.tests = workflow.test_stage.tests.filter(test => {
            return test.test_name !== testName
          })
        }
        updateWorkflowAPI(workflow).then(() => {
          this.$message({
            type: 'success',
            message: '移除关联成功'
          })
          this.fetchTestList()
        })
      }).catch(() => {
        this.$message({
          type: 'info',
          message: '已取消删除'
        })
      })
    },
    changeRoute (tab) {
      if (tab.name === 'function') {
        this.$router.replace(`/v1/${this.basePath}/detail/${this.projectName}/test/function`)
      }
    }
  },
  created () {
    this.activeTab = 'function'
    this.fetchTestList()
  },
  components: {
    functionTestList
  }
}
</script>

<style lang="less">
.function-test-manage {
  position: relative;
  flex: 1;
  padding: 15px 20px;
  overflow: auto;

  .module-title {
    h1 {
      margin-bottom: 1.5rem;
      font-weight: 200;
      font-size: 2rem;
    }
  }

  .link {
    color: #1989fa;
  }

  .delete-connection {
    color: #ff4949;
    cursor: pointer;
  }

  .add-connection {
    color: #1989fa;
    cursor: pointer;
  }

  .menu-item {
    margin-right: 15px;
    color: #000;
    font-size: 23px;
    cursor: pointer;
  }

  .breadcrumb {
    margin-bottom: 25px;

    .el-breadcrumb {
      font-size: 16px;
      line-height: 1.35;

      .el-breadcrumb__item__inner a:hover,
      .el-breadcrumb__item__inner:hover {
        color: #1989fa;
        cursor: pointer;
      }
    }
  }

  .add-test {
    margin-top: 15px;
  }
}
</style>
