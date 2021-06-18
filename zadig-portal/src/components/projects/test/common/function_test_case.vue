<template>
  <div class="function-test-case">
    <div class="filter-container">
      <span class="filter-title">
        <i class="iconfont iconfilter"></i>
      </span>
      <el-checkbox-group v-model="filteredLabels">
        <el-checkbox v-for="item in testResultLabels"
                     :label="item.value"
                     :key="item.value">{{item.text}}</el-checkbox>
      </el-checkbox-group>

    </div>

    <el-table :data="filteredTestCases"
              style="width: 100%;">
      <el-table-column label="测试用例描述">
        <template slot-scope="scope">

          <el-button v-if="checkStatus(scope.row)==='失败'"
                     @click="showFailureMeta(scope.$index)"
                     class="case-desc failure"
                     type="text">
            <span class="case-desc">{{ scope.row.tc_name }}</span>
          </el-button>
          <span v-else>{{ scope.row.tc_name }}</span>

          <div v-if="checkStatus(scope.row)==='失败' && showFailureMetaFlag[scope.$index]"
               class="fail-meta">{{scope.row.mergedOutput}}</div>
          <div v-if="checkStatus(scope.row)==='错误' && showFailureMetaFlag[scope.$index]"
               class="fail-meta">{{scope.row.mergedOutput}}</div>
        </template>
      </el-table-column>
      <el-table-column prop="scope.row"
                       label-class-name="filter-header"
                       label="测试结果"
                       width="100"
                       align="left">
        <template slot-scope="scope">

          <el-button v-if="checkStatus(scope.row)==='失败'"
                     type="text"
                     class="fail-btn"
                     @click="showFailureMeta(scope.$index)">
            <span>
              {{ checkStatus(scope.row) }}
            </span>
            <span
                  :class="`el-icon-caret-${showFailureMetaFlag[scope.$index] ? 'top' : 'bottom'} icon`"></span>
          </el-button>
          <el-button v-else-if="checkStatus(scope.row)==='错误'"
                     type="text"
                     class="error-btn"
                     @click="showFailureMeta(scope.$index)">
            <span>
              {{ checkStatus(scope.row) }}
            </span>
            <span
                  :class="`el-icon-caret-${showFailureMetaFlag[scope.$index] ? 'top' : 'bottom'} icon`"></span>
          </el-button>
          <span v-else>
            {{ checkStatus(scope.row) }}
          </span>

        </template>
      </el-table-column>
      <el-table-column label="运行时间(s)"
                       width="110"
                       align="center">
        <template slot-scope="scope">
          <span>{{ scope.row.time.toFixed(2) }}</span>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script>

export default {
  data () {
    return {
      showFailureMetaFlag: {},
      filteredLabels: [],
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
          text: '错误',
          value: 'error'
        },
        {
          text: '未执行',
          value: 'skipped'
        }
      ]
    }
  },
  methods: {
    checkStatus (obj) {
      if (obj.failure !== null) {
        return '失败'
      } else if (typeof obj.failure === 'string') {
        return '失败'
      } else if (obj.skipped !== null) {
        return '未执行'
      } else if (obj.error) {
        return '错误'
      } else {
        return '成功'
      }
    },
    showFailureMeta (index) {
      if (!this.showFailureMetaFlag[index]) {
        this.$set(this.showFailureMetaFlag, index, true)
      } else {
        this.$set(this.showFailureMetaFlag, index, false)
      }
    },
    filterTag (value, row) {
      if (value === 'failure' && row[value] !== null) {
        return true
      } else if (value === 'skipped' && row[value] !== null) {
        return true
      } else if (value === 'succeeded' && typeof row[value] === 'undefined' && !row.failure && !row.skipped && !row.error) {
        return true
      } else if (value === 'error' && row[value]) {
        return true
      }
    }
  },
  computed: {
    filteredTestCases () {
      if (this.filteredLabels.length === 0) {
        return this.testCases
      } else {
        // eslint-disable-next-line array-callback-return
        return this.testCases.filter(element => {
          if (this.filteredLabels.indexOf('failure') >= 0 && element.failure !== null) {
            return true
          } else if (this.filteredLabels.indexOf('skipped') >= 0 && element.skipped !== null) {
            return true
          } else if (this.filteredLabels.indexOf('succeeded') >= 0 && typeof element.succeeded === 'undefined' && !element.failure && !element.skipped && !element.error) {
            return true
          } else if (this.filteredLabels.indexOf('error') >= 0 && element.error) {
            return true
          }
        })
      }
    }
  },
  props: {
    testCases: {
      required: true,
      type: Array
    }
  }
}
</script>

<style lang="less">
.filter-container {
  margin: 2px 8px;

  .el-checkbox-group {
    display: inline-block;
  }

  .filter-title {
    i {
      color: #909399;
      font-weight: 500;
      font-size: 25px;
      line-height: 22px;
    }
  }
}

.fail-meta {
  padding: 6px;
  overflow: hidden;
  color: #f56c6c;
  font-size: 13px;
  font-family: Monaco, monospace;
  white-space: pre-wrap;
  word-wrap: break-word;
  background-color: #fafafa;
  border: 2px solid #f56c6c;
  border-radius: 4px;
  -webkit-transition: height 0.2s;
  transition: height 0.2s;
}

.fail-btn {
  span {
    color: #ff1949 !important;
  }
}

.error-btn {
  span {
    color: #e6a23c !important;
  }
}

.case-desc {
  white-space: normal;
  text-align: left;

  &.failure {
    color: #ff1949;
  }
}
</style>
