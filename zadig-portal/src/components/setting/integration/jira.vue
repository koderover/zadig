<template>
  <div class="integration-jira-container">

    <!--start of edit jira dialog-->
    <el-dialog title="Jira 配置-修改"
               :close-on-click-modal="false"
               custom-class="edit-form-dialog"
               :visible.sync="dialogJiraEditFormVisible">
      <el-form :model="jiraEdit"
               @submit.native.prevent
               label-position="top"
               :rules="jiraRules"
               ref="jiraEditForm">
        <el-form-item label="Jira 地址"
                      label-width="80px"
                      prop="host">
          <el-input v-model="jiraEdit.host"
                    placeholder="企业 Jira 地址"
                    autofocus
                    auto-complete="off"></el-input>
        </el-form-item>
        <el-form-item label="用户名"
                      label-width="80px"
                      prop="user">
          <el-input v-model="jiraEdit.user"
                    placeholder="有读写 Issue 权限的用户"
                    autofocus
                    auto-complete="off"></el-input>
        </el-form-item>
        <el-form-item label="密码"
                      label-width="180px"
                      prop="accessToken">
          <el-input v-model="jiraEdit.accessToken"
                    placeholder="用户密码"
                    autofocus
                    auto-complete="off"></el-input>
        </el-form-item>
      </el-form>
      <div slot="footer"
           class="dialog-footer">
        <el-button type="primary"
                   native-type="submit"
                   size="small"
                   @click="updateJiraConfig()"
                   class="start-create">确定</el-button>
        <el-button plain
                   native-type="submit"
                   size="small"
                   @click="handleJiraCancel()">取消</el-button>
      </div>
    </el-dialog>
    <!--end of edit jira dialog-->

    <!--start of edit jira dialog-->
    <el-dialog title="Jira 配置-添加"
               :close-on-click-modal="false"
               custom-class="edit-form-dialog"
               :visible.sync="dialogJiraAddFormVisible">
      <el-form :model="jiraAdd"
               @submit.native.prevent
               :rules="jiraRules"
               label-position="top"
               ref="jiraAddForm">
        <el-form-item label="Jira 地址"
                      label-width="80px"
                      prop="host">
          <el-input v-model="jiraAdd.host"
                    placeholder="企业 Jira 地址"
                    autofocus
                    auto-complete="off"></el-input>
        </el-form-item>
        <el-form-item label="用户名"
                      label-width="80px"
                      prop="user">
          <el-input v-model="jiraAdd.user"
                    placeholder="有读写 Issue 权限的用户"
                    autofocus
                    auto-complete="off"></el-input>
        </el-form-item>
        <el-form-item label="密码"
                      prop="accessToken">
          <el-input v-model="jiraAdd.accessToken"
                    placeholder="用户密码"
                    autofocus
                    auto-complete="off"></el-input>
        </el-form-item>
      </el-form>
      <div slot="footer"
           class="dialog-footer">
        <el-button type="primary"
                   native-type="submit"
                   size="small"
                   @click="createJiraConfig()"
                   class="start-create">确定</el-button>
        <el-button plain
                   native-type="submit"
                   size="small"
                   @click="handleJiraCancel()">取消</el-button>
      </div>
    </el-dialog>
    <!--end of edit jira dialog-->
    <div class="tab-container">
          <template>
            <el-alert type="info"
                      :closable="false"
                      description="为系统定义 Jira 集成，配置后工作流可以追踪到 Jira Issue">
            </el-alert>
          </template>
          <div class="sync-container">
            <el-button v-if="jira.length === 0"
                       size="small"
                       type="primary"
                       plain
                       @click="handleJiraAdd">添加</el-button>
          </div>
          <el-table :data="jira"
                    style="width: 100%;">
            <el-table-column label="Jira 地址">
              <template slot-scope="scope">
                {{scope.row.host}}
              </template>
            </el-table-column>
            <el-table-column label="用户名">
              <template slot-scope="scope">
                {{scope.row.user}}
              </template>
            </el-table-column>
            <el-table-column label="密码">
              <template>
                **********
              </template>
            </el-table-column>
            <el-table-column label="操作"
                             width="160">
              <template slot-scope="scope">
                <el-button type="primary"
                           size="mini"
                           plain
                           @click="handleJiraEdit(scope.row)">编辑</el-button>
                <el-button type="danger"
                           size="mini"
                           @click="handleJiraDelete"
                           plain>删除</el-button>
              </template>
            </el-table-column>
          </el-table>
    </div>
  </div>
</template>
<script>
import {
  getJiraAPI, updateJiraAPI, deleteJiraAPI, createJiraAPI
} from '@api'
export default {
  data () {
    return {
      tabPosition: 'top',
      activeTab: '',
      jira: [],
      jiraAdd: {
        host: '',
        user: '',
        accessToken: ''
      },
      jiraEdit: {
        host: '',
        user: '',
        accessToken: ''
      },
      jiraRules: {
        user: {
          required: true,
          message: '请输入用户名',
          trigger: ['blur', 'change']
        },
        host: [{
          required: true,
          message: '请输入 Host',
          trigger: 'blur'
        },
        {
          type: 'url',
          message: '请输入正确的 URL，包含协议',
          trigger: ['blur', 'change']
        }],
        accessToken: {
          required: true,
          message: '请输入密码',
          trigger: ['blur', 'change']
        }
      },
      dialogJiraAddFormVisible: false,
      dialogJiraEditFormVisible: false
    }
  },
  methods: {
    clearValidate (ref) {
      this.$refs[ref].clearValidate()
    },
    getJiraConfig () {
      const id = this.currentOrganizationId
      getJiraAPI(id).then((res) => {
        if (!res.resultCode) {
          this.$set(this.jira, [0], res)
        } else {
          this.$set(this, 'jira', [])
        }
      })
    },
    handleJiraAdd () {
      this.dialogJiraAddFormVisible = true
    },
    handleJiraEdit (row) {
      this.dialogJiraEditFormVisible = true
      this.jiraEdit = this.$utils.cloneObj(row)
    },
    handleJiraDelete () {
      this.$confirm(`确定要删除这个 Jira 配置吗？`, '确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        const id = this.currentOrganizationId
        deleteJiraAPI(id).then((res) => {
          this.getJiraConfig()
          this.$message({
            message: 'Jira 配置删除成功',
            type: 'success'
          })
        })
      })
    },
    createJiraConfig () {
      this.$refs.jiraAddForm.validate((valid) => {
        if (valid) {
          const id = this.currentOrganizationId
          const payload = this.jiraAdd
          createJiraAPI(id, payload).then((res) => {
            this.getJiraConfig()
            this.handleJiraCancel()
            this.$message({
              message: 'Jira 配置添加成功',
              type: 'success'
            })
          })
        } else {
          return false
        }
      })
    },
    updateJiraConfig () {
      this.$refs.jiraEditForm.validate((valid) => {
        if (valid) {
          const id = this.currentOrganizationId
          const payload = this.jiraEdit
          updateJiraAPI(id, payload).then((res) => {
            this.getJiraConfig()
            this.handleJiraCancel()
            this.$message({
              message: 'Jira 配置修改成功',
              type: 'success'
            })
          })
        } else {
          return false
        }
      })
    },
    handleJiraCancel () {
      if (this.$refs.jiraAddForm) {
        this.$refs.jiraAddForm.resetFields()
        this.dialogJiraAddFormVisible = false
      }
      if (this.$refs.jiraEditForm) {
        this.$refs.jiraEditForm.resetFields()
        this.dialogJiraEditFormVisible = false
      }
    }
  },
  computed: {
    currentOrganizationId () {
      return this.$store.state.login.userinfo.organization.id
    }
  },
  activated () {
    this.getJiraConfig()
  }
}
</script>

<style lang="less">
.integration-jira-container {
  position: relative;
  flex: 1;
  overflow: auto;
  font-size: 13px;

  .module-title h1 {
    margin-bottom: 1.5rem;
    font-weight: 200;
    font-size: 2rem;
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

  .tab-container {
    .sync-container {
      padding-top: 15px;
      padding-bottom: 15px;
    }
  }

  .text-success {
    color: rgb(82, 196, 26);
  }

  .text-failed {
    color: #ff1949;
  }

  .edit-form-dialog {
    width: 550px;

    .el-dialog__header {
      padding: 15px;
      text-align: center;
      border-bottom: 1px solid #e4e4e4;

      .el-dialog__close {
        font-size: 10px;
      }
    }

    .el-dialog__body {
      padding: 0 20px;
      color: #606266;
      font-size: 14px;

      .el-form-item {
        margin-bottom: 15px;
      }
    }

    .el-select {
      width: 100%;
    }

    .el-input {
      display: inline-block;
    }
  }
}
</style>
