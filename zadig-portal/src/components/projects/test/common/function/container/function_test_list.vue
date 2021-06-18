<template>
  <div class="function-test-list">
    <el-table :data="testList">
      <el-table-column prop="name"
                       label="测试用例集名称">
        <template slot-scope="scope">
          <router-link :to="`${jobPrefix}/${scope.row.name}-job`">
            <span class="link">{{scope.row.name}}</span>
          </router-link>
        </template>
      </el-table-column>
      <el-table-column width="100px"
                       prop="test_case_num"
                       label="用例数量">
        <template slot-scope="scope">
          <span v-if="scope.row.test_case_num">{{ scope.row.test_case_num }}</span>
          <span v-else>N/A</span>
        </template>
      </el-table-column>

      <el-table-column width="100px"
                       prop="execute_num"
                       label="执行次数">
        <template slot-scope="scope">
          <span v-if="scope.row.execute_num">{{ scope.row.execute_num }}</span>
          <span v-else>N/A</span>
        </template>
      </el-table-column>

      <el-table-column width="100px"
                       prop="pass_rate"
                       label="通过率">
        <template slot-scope="scope">
          <span v-if="scope.row.pass_rate">{{ (scope.row.pass_rate *100).toFixed(2)+'%'  }}</span>
          <span v-else>N/A</span>
        </template>
      </el-table-column>

      <el-table-column width="100px"
                       prop="avg_duration"
                       label="平均耗时">
        <template slot-scope="scope">
          <span
                v-if="scope.row.avg_duration">{{ $utils.timeFormat(scope.row.avg_duration.toFixed(1))}}</span>
          <span v-else>N/A</span>
        </template>
      </el-table-column>

      <el-table-column prop="workflows"
                       label="关联的工作流">
        <template slot="header"
                  >
          <span>已关联的工作流</span>
        </template>
        <template slot-scope="scope">
          <div v-if="scope.row.workflows">
            <div v-for="(workflow,index) in scope.row.workflows"
                 :key="index"
                 class="info-wrapper">
              <router-link class="link"
                           :to="`/v1/projects/detail/${projectName}/pipelines/multi/${workflow.name}`">
                {{workflow.name}}
              </router-link>
              <span @click="deleteConnection(scope.row.name,workflow)"
                    class="delete-connection el-icon-remove"></span>
            </div>
          </div>
          <span @click="addConnection(scope.row.name)"
                class="add-connection el-icon-circle-plus">添加</span>
        </template>
      </el-table-column>
      <el-table-column label="操作"
                       width="200px">
        <template slot-scope="scope">
          <span class="menu-item el-icon-video-play"
                @click="runTests(scope.row.name)"><span style="font-size: 18px;"> 执行</span></span>
          <router-link
                       :to="`/v1/${basePath}/detail/${scope.row.product_name}/test/function/${scope.row.name}`">
            <span class="menu-item el-icon-setting"></span>
          </router-link>
          <el-dropdown>
            <span class="el-dropdown-link">
              <i class="menu-item  el-icon-s-operation"></i>
            </span>
            <el-dropdown-menu slot="dropdown">
              <el-dropdown-item @click.native="removeTest(scope.row)">删除
              </el-dropdown-item>
            </el-dropdown-menu>
          </el-dropdown>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script type="text/javascript">
export default {
  data () {
    return {

    }
  },
  props: {
    testList: {
      required: true,
      type: Array
    },
    projectName: {
      required: true,
      type: String
    },
    jobPrefix: {
      required: true,
      type: String
    },
    inProject: {
      required: false,
      type: Boolean,
      default: false
    },
    basePath: {
      required: false,
      type: String,
      default: 'projects'
    }
  },
  methods: {
    removeTest (row) {
      this.$emit('removeTest', row)
    },
    addConnection (testName) {
      this.$emit('addConnection', testName)
    },
    deleteConnection (testName, workflow) {
      this.$emit('deleteConnection', testName, workflow)
    },
    runTests (row) {
      this.$emit('runTests', row)
    }
  }
}
</script>

<style lang="less">
.function-test-list {
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
}
</style>
