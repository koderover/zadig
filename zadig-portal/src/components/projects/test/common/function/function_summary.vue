<template>
    <div class="function-summary">
      <el-card class="box-card wide"
               :body-style="{ padding: '0px', margin: '15px 0 0 0' }">
        <div slot="header"
             class="block-title">
          最新一次测试报告
        </div>
        <div class="text item">
          <el-row>
            <el-col :span="12">
              <div>
                <el-row :gutter="0">
                  <el-col :span="6">
                    <div class="item-title">总测试用例</div>
                  </el-col>
                  <el-col :span="5">
                    <div class="item-desc">{{latestTestSummary.tests }}
                    </div>
                  </el-col>

                  <el-col :span="6">
                    <div class="item-title">成功用例</div>
                  </el-col>
                  <el-col :span="5">
                    <div class="">
                      <div class="item-desc">{{latestTestSummary.tests -
                        latestTestSummary.failures - latestTestSummary.errors}}</div>
                    </div>
                  </el-col>
                </el-row>

                <el-row :gutter="0">
                  <el-col :span="6">
                    <div class="item-title">失败用例</div>
                  </el-col>
                  <el-col :span="5">
                    <div class="item-desc">{{latestTestSummary.failures}}</div>
                  </el-col>
                  <el-col :span="6">
                    <div class="item-title">错误用例</div>
                  </el-col>
                  <el-col :span="5">
                    <div class="item-desc">{{latestTestSummary.errors}}</div>
                  </el-col>
                </el-row>
                <el-row :gutter="0">
                  <el-col :span="6">
                    <div class="item-title">未执行用例</div>
                  </el-col>

                  <el-col :span="5">
                    <div class="item-desc">{{latestTestSummary.skips}}</div>
                  </el-col>

                  <el-col :span="6">
                    <div class=" item-title">测试用时</div>
                  </el-col>
                  <el-col :span="5">
                    <div class=" item-desc">
                      {{$utils.timeFormat(parseInt((latestTestSummary.time)))}}
                    </div>
                  </el-col>
                </el-row>
              </div>
            </el-col>
          </el-row>
        </div>
      </el-card>
      <el-button :icon="!showTestCase?'el-icon-caret-bottom':'el-icon-caret-top'"
                 type="primary"
                 size="small"
                 @click="showTestCase = !showTestCase"
                 plain>查看用例</el-button>
      <el-collapse-transition>
        <el-card v-show="showTestCase"
                 class="box-card task-process"
                 :body-style="{ padding: '0px', margin: '15px 0 0 0' }">
          <div slot="header"
               class="clearfix">
            <span class="block-title">详细用例（可滚动查看）</span>
          </div>
          <function-test-case :testCases="testCases"></function-test-case>
        </el-card>
      </el-collapse-transition>
      <el-card class="box-card full"
               :body-style="{ padding: '0px', margin: '15px 0 30px 0' }">
        <div slot="header"
             class="block-title">
          历史任务
        </div>
        <task-list :taskList="workflowTasks"
                   :total="total"
                   :pageSize="pageSize"
                   :projectName="projectName"
                   :baseUrl="`/v1/${basePath}/detail/${projectName}/test/detail/function/${workflowName}`"
                   :workflowName="workflowName"
                   :functionTestBaseUrl="`/v1/${basePath}/detail/${projectName}/test/testcase/function/${workflowName}`"
                   @currentChange="changeTaskPage"
                   showTestReport>
        </task-list>
      </el-card>
    </div>
</template>

<script>
import functionTestCase from '@/components/projects/test/common/function_test_case.vue'
import { getLatestTestReportAPI, workflowTaskListAPI } from '@api'
export default {
  data () {
    return {
      latestTestSummary: {
        failures: 0,
        skips: 0,
        tests: 0,
        time: 0,
        errors: 0,
        successes: 0
      },
      testCases: [
        {
          name: '',
          skipped: '',
          time: 0
        }
      ],
      testResultLabels: [
        {
          text: '失败',
          value: 'failure'
        },
        {
          text: '成功',
          value: 'succeeded'
        },
        {
          text: '未执行',
          value: 'skipped'
        },
        {
          text: '错误',
          value: 'error'
        }
      ],
      workflow: {},
      workflowTasks: [],
      total: 0,
      pageSize: 50,
      showTestCase: false,
      guideDialog: false,
      taskDialogVisible: false,
      showFailureMetaFlag: {},
      forcedUserInput: {}
    }
  },
  props: {
    basePath: {
      request: true,
      type: String
    }
  },
  computed: {
    workflowName () {
      return this.$route.params.test_name
    },
    projectName () {
      return this.$route.params.project_name
    },
    serviceName () {
      return this.$route.params.test_name.slice(0, -4)
    }
  },
  methods: {
    getLatestTest () {
      getLatestTestReportAPI(this.serviceName).then(res => {
        this.latestTestSummary = res
        this.testCases = res.testcase
        this.testCases.forEach(testCase => {
          const blocks = []
          if (testCase.failure && typeof testCase.failure === 'string') {
            blocks.push(`失败原因:\n${testCase.failure}`)
          }
          if (testCase.failure && typeof testCase.failure === 'object') {
            blocks.push(`失败信息:\n${testCase.failure.message}`)
            blocks.push(`失败详情:\n${testCase.failure.text}`)
          }
          if (testCase.system_out) {
            blocks.push(`标准输出:\n${testCase.system_out}`)
          }
          if (testCase.error) {
            blocks.push(`错误信息:\n${testCase.error.message}`)
            blocks.push(`错误详情:\n${testCase.error.text}`)
            blocks.push(`错误类型:\n${testCase.error.type}`)
          }
          testCase.mergedOutput = blocks.join('\n')
        })
      })
    },
    fetchHistory (start, max) {
      workflowTaskListAPI(this.workflowName, start, max, 'test').then(res => {
        res.data.forEach(element => {
          if (element.test_reports) {
            const testArray = []
            for (const testName in element.test_reports) {
              const val = element.test_reports[testName]
              if (typeof val === 'object') {
                const struct = {
                  success: null,
                  total: null,
                  name: null,
                  type: null,
                  time: null,
                  img_id: null
                }
                if (val.functionTestSuite) {
                  struct.name = testName
                  struct.type = 'function'
                  struct.success = val.functionTestSuite.successes ? val.functionTestSuite.successes : (val.functionTestSuite.tests - val.functionTestSuite.failures - val.functionTestSuite.errors)
                  struct.total = val.functionTestSuite.tests
                  struct.time = val.functionTestSuite.time
                }
                testArray.push(struct)
              }
            }
            element.testSummary = testArray
          }
        })
        this.workflowTasks = res.data
        this.total = res.total
      })
    },
    changeTaskPage (val) {
      const start = (val - 1) * this.pageSize
      this.fetchHistory(start, this.pageSize)
    }

  },
  created () {
    this.getLatestTest()
    this.fetchHistory(0, this.pageSize)
  },
  components: {
    functionTestCase
  }
}
</script>

<style lang="less">
.function-summary {
  position: relative;
  flex: 1;
  padding: 0 30px;
  overflow: auto;
  font-size: 13px;
  background-color: #fff;

  .text {
    font-size: 13px;
  }

  .item {
    padding: 10px 0;
    padding-left: 1px;

    .icon-color {
      color: #9ea3a9;
      cursor: pointer;

      &:hover {
        color: #1989fa;
      }
    }

    .icon-color-cancel {
      color: #ff4949;
      cursor: pointer;
    }
  }

  .clearfix::before,
  .clearfix::after {
    display: table;
    content: "";
  }

  .block-title {
    color: #999;
    font-size: 16px;
    line-height: 20px;
  }

  .clearfix::after {
    clear: both;
  }

  .box-card {
    width: 100%;
    background-color: #fff;

    .item-title {
      color: #8d9199;
    }

    .process {
      line-height: 24px;
    }

    .operation {
      line-height: 18px;
    }

    .item-desc {
      .start-build,
      .edit-pipeline {
        margin-right: 0.3em;
        font-size: 1.3rem;
        cursor: pointer;

        &:hover {
          color: #1989fa;
        }
      }

      .favorite {
        display: inline-block;
        color: #69696bb3;
        cursor: pointer;

        &.liked {
          color: #f4e118;
        }

        &:hover {
          color: #f4e118;
        }
      }
    }

    .task-id,
    .report-link {
      color: #1989fa;
    }

    .pagination {
      display: flex;
      align-items: center;
      justify-content: center;
      margin-top: 20px;
    }
  }

  .box-card,
  .box-card-stack {
    margin-top: 15px;
    border: none;
    box-shadow: none;
  }

  .wide {
    width: 65%;
  }

  .full {
    width: 100%;
  }

  .el-card__header {
    padding-left: 0;
  }

  .el-row {
    margin-bottom: 15px;

    &:last-child {
      margin-bottom: 0;
    }
  }

  .pipeline-edit {
    .el-dialog__body {
      padding: 15px 20px;

      .el-form {
        .el-form-item {
          margin-bottom: 15px;
        }
      }
    }
  }

  .fileTree-dialog {
    .el-dialog__body {
      padding: 0 5px;
    }
  }

  .buildv2-edit-form {
    .el-form-item__label {
      padding: 0;
      font-size: 13px;
      line-height: 25px;
    }
  }

  .not-anchor {
    color: unset;
  }

  .run-workflow {
    .el-dialog__body {
      padding: 8px 10px;
      color: #606266;
      font-size: 14px;
    }
  }
}
</style>
