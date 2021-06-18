<template>
  <div class="product-test">
    <el-card class="box-card">
      <div class="section-head">
        自动化测试
        <el-switch v-model="testEnabled">
        </el-switch>

      </div>
      <template v-if="testEnabled">
        <el-table :data="testConfigs">
          <el-table-column prop="name"
                           label="测试名称"></el-table-column>
          <el-table-column prop="product_name"
                           label="项目名称"></el-table-column>
          <el-table-column prop="desc"
                           label="描述"></el-table-column>
          <el-table-column label="环境变量">
            <template slot-scope="{ row }">
              <el-popover placement="left"
                          width="450"
                          trigger="click">
                <template slot="reference">
                  <el-button type="text"
                             style="padding-left: 5px;">设置</el-button>
                </template>
                <el-table :data="row.envs">
                  <el-table-column prop="key"
                                   label="Key"></el-table-column>
                  <el-table-column label="Value">
                    <template slot-scope="{ row }">
                      <el-input v-model="row.value"></el-input>
                    </template>
                  </el-table-column>
                </el-table>
              </el-popover>
            </template>
          </el-table-column>
          <el-table-column label="操作"
                           width="100px">
            <template slot-scope="scope">
              <el-button @click="removeTest(scope.$index)"
                         type="danger"
                         icon="el-icon-delete"
                         size="mini">删除</el-button>
            </template>
          </el-table-column>
        </el-table>

        <div class="test-adder">
          <el-select v-model="testToAdd"
                     filterable
                     size="small">
            <el-option v-for="test of unConfiguredTest"
                       :key="test.name"
                       :label="`${test.name}/${test.product_name}`"
                       :value="test.name"></el-option>
          </el-select>
          <el-button @click="addTestConfig"
                     type="default"
                     size="small"
                     icon="el-icon-plus">添加测试</el-button>
        </div>

      </template>
    </el-card>
  </div>
</template>

<script type="text/javascript">
import bus from '@utils/event_bus'
import { testDetailAPI } from '@api'

export default {
  data () {
    return {
      testList: [],
      testToAdd: '',
      unConfiguredTest: []
    }
  },
  computed: {
    testMap () {
      return this.$utils.arrayToMap(this.testList, 'name')
    },
    testEnabled: {
      get () {
        return this.test_stage.enabled
      },
      set (val) {
        this.test_stage.enabled = val
      }
    },
    testNames () {
      return (this.test_stage && this.test_stage.tests && this.test_stage.tests.map(t => { return t.test_name })) || []
    },
    // NOTE: testConfigs只用于显示，要修改，修改testNames
    testConfigs () {
      const test = []
      this.testNames.forEach(name => {
        if (this.testMap[name]) {
          test.push(this.testMap[name])
        }
      })
      return test
    }
  },
  watch: {
    product_tmpl_name (newVal, oldVal) {
      // product template变动时，更新test list，清空配置，重置testToAdd
      testDetailAPI().then(res => {
        if (this.test_stage && this.test_stage.tests && this.test_stage.tests.length > 0) {
          const resTests = this.$utils.arrayToMap(res, 'name')
          const testsHad = []
          this.test_stage.tests.forEach(t => {
            if (!resTests[t.test_name]) {
              return
            }
            if (t.envs.length > 0) {
              // 如果有envs内容 修改res的envs内容 和 envs的内容
              // 这里会因为增删key需要作出改变
              const envObjs = this.$utils.arrayToMap(t.envs, 'key')
              resTests[t.test_name].envs.forEach(env => {
                env.value = (envObjs[env.key] && envObjs[env.key].value) || env.value
              })
            }
            testsHad.push({
              test_name: t.test_name,
              envs: resTests[t.test_name].envs || []
            })
          })
          this.test_stage.tests = testsHad
        }
        this.testList = res
        this.unConfiguredTest = this.testList.filter(item => { return !this.testNames.includes(item.name) })
        this.testToAdd = ''
      })
      // 修改前是空就不清，否则edit时load出来的数据会被错误地清空
      if (oldVal) {
        this.$set(this.test_stage, 'tests', [])
      }
      if (!this.test_stage.tests) {
        this.$set(this.test_stage, 'tests', [])
      }
    },
    testNames (newVal, oldVal) {
      this.unConfiguredTest = this.testList.filter(item => { return !newVal.includes(item.name) })
    }
  },
  props: {
    test_stage: {
      required: true,
      type: Object
    },
    editMode: {
      required: true,
      type: Boolean
    },
    product_tmpl_name: {
      required: true,
      type: String
    }
  },
  methods: {
    addTestConfig () {
      if (this.testToAdd) {
        this.test_stage.tests.push({
          test_name: this.testToAdd,
          envs: this.testMap[this.testToAdd].envs
        })
        this.testToAdd = ''
      }
    },
    removeTest (index) {
      this.test_stage.tests.splice(index, 1)
    }
  },
  created () {
    bus.$on('check-tab:test', () => {
      bus.$emit('receive-tab-check:test', true)
    })
  },
  beforeDestroy () {
    bus.$off('check-tab:test')
  }
}
</script>

<style lang="less">
.product-test {
  .test-adder {
    margin-top: 20px;
  }

  .section-head {
    width: 222px;
    height: 28px;
    margin: 10px 0;
    color: #303133;
    font-size: 16px;
    line-height: 28px;
    border-bottom: 1px solid #eee;
  }
}
</style>
