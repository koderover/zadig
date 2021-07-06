<template>
  <div class="config-quota-container">

    <template>
      <el-alert type="info"
                :closable="false"
                description="设置系统中的组件配额">
      </el-alert>
    </template>
    <div class="quota-container">
      <section>
        <h1>历史任务保留策略 <el-tooltip effect="dark"
                      content="包括工作流任务及其产生的构建日志、测试日志、测试报告、二进制文件"
                      placement="right">
            <i class="el-icon-question tooltip"></i>
          </el-tooltip>
        </h1>
        <div class="config-list">
          <el-radio-group v-model="selectType">
            <el-radio style="display: block; margin-bottom: 10px;"
                      label="max_days">
              <span class="desc">保留天数</span>
              <el-input size="mini"
                        :disabled="selectType!=='max_days'"
                        v-model.number="WorkflowTaskRetention.max_days"
                        class="remain-day"
                        placeholder="">
                <template slot="append">天</template>
              </el-input>
            </el-radio>
            <el-radio style="display: block;"
                      label="max_items">
              <span class="desc">保留最近任务个数</span>
              <el-input size="mini"
                        :disabled="selectType!=='max_items'"
                        v-model.number="WorkflowTaskRetention.max_items"
                        class="remain-day"
                        placeholder="">
                <template slot="append">个</template>
              </el-input>
            </el-radio>
          </el-radio-group>
        </div>

      </section>
      <section class="operation">
        <el-button @click="setCapacity('WorkflowTaskRetention',selectType,WorkflowTaskRetention[selectType])"
                   size="small"
                   type="primary">确定</el-button>
      </section>
    </div>
  </div>
</template>
<script>
import { getCapacityAPI, setCapacityAPI } from '@api'
import bus from '@utils/event_bus'
export default {
  data () {
    return {
      selectType: '',
      WorkflowTaskRetention: {
        max_days: null,
        max_items: 1000
      }
    }
  },
  methods: {
    getCapacity (target) {
      getCapacityAPI(target).then((res) => {
        if (res.retention.max_days > 0) {
          this[target].max_days = res.retention.max_days
          this.selectType = 'max_days'
        } else if (res.retention.max_items > 0) {
          this[target].max_items = res.retention.max_items
          this.selectType = 'max_items'
        }
      })
    },
    setCapacity (target, key, value) {
      if (!isNaN(value) && value > 0) {
        const type = this.selectType
        const countStr = (type === 'max_days' ? `最新 ${this.WorkflowTaskRetention.max_days} 天` : `最近 ${this.WorkflowTaskRetention.max_items} 个`)
        this.$confirm(`只保留${countStr}任务产生的数据（包括构建日志、构建产生的 Tar 包、测试日志和测试报告），其他历史任务数据将被永久删除<br><span style="color:#ff1949;font-size:13px">注意：确定后，立即生效</span>`, '提示', {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          dangerouslyUseHTMLString: true,
          type: 'warning'
        }).then(() => {
          const payload = {
            target: target,
            retention: { [key]: value }
          }
          setCapacityAPI(payload).then((res) => {
            this.$message.success('更新系统配额成功')
            this.getCapacity('WorkflowTaskRetention')
          })
        }).catch(() => {
          this.$message({
            type: 'info',
            message: '已取消更新'
          })
        })
      } else {
        this.$message({
          type: 'error',
          message: '输入的数据不符合要求'
        })
      }
    }
  },
  computed: {

  },
  watch: {

  },

  created () {
    bus.$emit(`set-topbar-title`, { title: '系统配置', breadcrumb: [] })
    bus.$emit(`set-sub-sidebar-title`, {
      title: '',
      routerList: []
    })
    this.getCapacity('WorkflowTaskRetention')
  }
}
</script>

<style lang="less">
.config-quota-container {
  position: relative;
  flex: 1;
  overflow: auto;
  font-size: 13px;

  .quota-container {
    margin-top: 15px;
    padding: 20px 20px 20px 20px;
    border: 1px solid #eee;
    border-radius: 5px;

    h1 {
      font-size: 1rem;
      line-height: 1;

      .tooltip {
        color: #909399;
        cursor: pointer;
      }
    }

    .config-list {
      margin: 15px 0;

      .item {
        color: #303133;
        font-size: 16px;
      }

      span.desc {
        display: inline-block;
        width: 130px;
        color: #606266;
        font-weight: 500;
        font-size: 14px;

        &.concurrent {
          width: 150px;
        }
      }

      .el-input.remain-day {
        width: 140px;
      }
    }

    .operation {
      margin-top: 20px;
    }
  }
}
</style>
