<template>
  <div class="tab-container">
    <el-alert
      type="info"
      :closable="false"
      description="为系统配置 Jenkins server，配置后的服务可以使用 Jenkins job 构建镜像"
    >
    </el-alert>
    <div class="sync-container">
      <el-button
        v-if="tableData.length === 0"
        size="small"
        type="primary"
        plain
        @click="handleJenkinsaAdd"
        >添加</el-button
      >
    </div>
    <Etable :tableColumns="tableColumns" :tableData="tableData" id="id" />
    <AddJenkins :getJenkins="getJenkins" ref="jenkinsref" />
  </div>
</template>
<script>
import Etable from '@/components/common/etable'
import AddJenkins from './components/add_jenkins'
import { queryJenkins, deleteJenkins } from '@/api'
import _ from 'lodash'
export default {
  name: 'jenkins',
  components: {
    Etable,
    AddJenkins
  },
  data() {
    return {
      tableColumns: [
        {
          prop: 'url',
          label: '服务地址',
        },
        {
          prop: 'username',
          label: '用户名',
        },
        {
          prop: 'password',
          label: 'API token',
          render: () => {
            return (<div>**********</div>)
          }
        },
        {
          label: '操作',
          render: (scope) => {
            return (
              <div>
                <el-button type="primary" size="mini" onClick={()=>{this.handleJenkinsaEdit(scope.row)}} plain>
                  编辑
                </el-button>
                <el-button type="danger" size="mini" onClick={()=>{this.handleJenkinsaDelete(scope.row)}} plain>
                  删除
                </el-button>
              </div>
            )
          },
        },
      ],
      tableData: [],
    }
  },
  methods: {
    async handleJenkinsaDelete (data) {
      this.$confirm(`确定要删除这个代码源吗？`, '确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        deleteJenkins(data).then(() =>{
          this.$message.success('删除成功')
          this.getJenkins()
        })
      });
    },
    handleJenkinsaAdd () {
      this.$refs.jenkinsref.openDialog()
    },
    handleJenkinsaEdit (data) {
      this.$refs.jenkinsref.openDialog(_.cloneDeep(data))
    },
    async getJenkins (){
      let res = await queryJenkins().catch(error=> console.log(error))
      if(res) {
         this.tableData = res 
      }
    }
  },
  activated() {
    this.getJenkins()
  },
}
</script>
<style lang="less" scoped>
.tab-container {
  .sync-container {
    padding-top: 15px;
    padding-bottom: 15px;
  }
}
</style>