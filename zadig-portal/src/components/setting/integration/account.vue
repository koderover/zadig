<template>
  <div class="integration-account-container">
    <!--start of edit account dialog-->
    <el-dialog title="用户管理-编辑"
               custom-class="edit-form-dialog"
               :close-on-click-modal="false"
               :visible.sync="dialogUserAccountEditFormVisible">
      <el-form :model="userAccountEdit"
               @submit.native.prevent
               :rules="userAccountRules"
               status-icon
               ref="userAccountUpdateForm">
        <template v-if="userAccountEdit.type==='ad'||userAccountEdit.type==='ldap'">
          <el-form-item label="主机名"
                        prop="address">
            <el-input v-model="userAccountEdit.address"
                      placeholder="主机名"
                      autofocus
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item label="端口"
                        prop="port">
            <el-input-number v-model="userAccountEdit.port"
                             size="medium"
                             label="port"
                             :min="0"
                             :max="65535"></el-input-number>
          </el-form-item>
          <el-form-item label="用户名"
                        prop="username">
            <el-input v-model="userAccountEdit.username"
                      placeholder="用户名"
                      autofocus
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item label="密码"
                        prop="password">
            <el-input v-model="userAccountEdit.password"
                      placeholder="密码"
                      autofocus
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item label="DN"
                        prop="dn">
            <el-input v-model="userAccountEdit.dn"
                      placeholder="DN"
                      autofocus
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item label="userFilter"
                        prop="userFilter">
            <el-input v-model="userAccountEdit.userFilter"
                      placeholder="userFilter"
                      autofocus
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item label="groupFilter"
                        prop="groupFilter">
            <el-input v-model="userAccountEdit.groupFilter"
                      placeholder="groupFilter"
                      autofocus
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item label="TLS"
                        prop="isTLS">
            <el-checkbox v-model="userAccountEdit.isTLS">启用</el-checkbox>
          </el-form-item>
        </template>
        <template v-else>
          <el-form-item label="Client Id"
                        prop="clientId">
            <el-input v-model="userAccountEdit.clientId"
                      placeholder="Client Id"
                      autofocus
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item label="Secret"
                        prop="secret">
            <el-input v-model="userAccountEdit.secret"
                      placeholder="Secret"
                      autofocus
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item label="Redirect"
                        prop="redirect">
            <el-input v-model="userAccountEdit.redirect"
                      placeholder="Redirect"
                      autofocus
                      auto-complete="off"></el-input>
          </el-form-item>
        </template>
      </el-form>
      <div slot="footer"
           class="dialog-footer">
        <el-button v-if="userAccountEdit.type==='ad'||userAccountEdit.type==='ldap'"
                   type="primary"
                   native-type="submit"
                   size="small"
                   @click="updateAccountUser()"
                   class="start-create">测试并保存</el-button>
        <el-button v-else
                   type="primary"
                   native-type="submit"
                   size="small"
                   @click="updateSSO()"
                   class="start-create">保存</el-button>
        <el-button plain
                   native-type="submit"
                   size="small"
                   @click="dialogUserAccountEditFormVisible = false">取消</el-button>
      </div>
    </el-dialog>
    <!--end of edit account dialog-->

    <!--start of add account dialog-->
    <el-dialog title="用户管理-添加"
               :close-on-click-modal="false"
               custom-class="edit-form-dialog"
               :visible.sync="dialogUserAccountAddFormVisible">
      <el-form :model="userAccountAdd"
               @submit.native.prevent
               :rules="userAccountRules"
               status-icon
               ref="userAccountForm">
        <el-form-item label="账号系统"
                      prop="type">
          <el-select v-model="userAccountAdd.type"
                     @change="clearValidate('userAccountForm')">
            <el-option label="AD"
                       value="ad"></el-option>
            <el-option label="LDAP"
                       value="ldap"></el-option>
            <el-option label="SSO"
                       value="sso"></el-option>
          </el-select>
        </el-form-item>
        <template v-if="userAccountAdd.type==='ad'||userAccountAdd.type==='ldap'">
          <el-form-item label="主机名"
                        prop="address">
            <el-input v-model="userAccountAdd.address"
                      placeholder="主机名"
                      autofocus
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item label="端口"
                        prop="port">
            <el-input-number v-model="userAccountAdd.port"
                             size="medium"
                             label="port"
                             :min="0"
                             :max="65535"></el-input-number>
          </el-form-item>
          <el-form-item label="用户名"
                        prop="username">
            <el-input v-model="userAccountAdd.username"
                      placeholder="用户名"
                      autofocus
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item label="密码"
                        prop="password">
            <el-input v-model="userAccountAdd.password"
                      placeholder="密码"
                      autofocus
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item label="DN"
                        prop="dn">
            <el-input v-model="userAccountAdd.dn"
                      placeholder="DN"
                      autofocus
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item label="userFilter"
                        prop="userFilter">
            <el-input v-model="userAccountAdd.userFilter"
                      placeholder="userFilter"
                      autofocus
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item label="groupFilter"
                        prop="groupFilter">
            <el-input v-model="userAccountAdd.groupFilter"
                      placeholder="groupFilter"
                      autofocus
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item label="TLS"
                        prop="isTLS">
            <el-checkbox v-model="userAccountAdd.isTLS">启用</el-checkbox>
          </el-form-item>
        </template>
        <template v-if="userAccountAdd.type==='sso'">
          <el-form-item label="Client Id"
                        prop="clientId">
            <el-input v-model="userAccountAdd.clientId"
                      placeholder="Client Id"
                      autofocus
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item label="Secret"
                        prop="secret">
            <el-input v-model="userAccountAdd.secret"
                      placeholder="Secret"
                      autofocus
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item label="Redirect"
                        prop="redirect">
            <el-input v-model="userAccountAdd.redirect"
                      placeholder="Redirect"
                      autofocus
                      auto-complete="off"></el-input>
          </el-form-item>
        </template>
      </el-form>
      <div slot="footer"
           class="dialog-footer">
        <el-button v-if="userAccountAdd.type!==''"
                   type="primary"
                   native-type="submit"
                   size="small"
                   @click="createAccountUser()"
                   class="start-create">{{userAccountAdd.type==='sso'?'保存':'测试并保存'}}</el-button>
        <el-button plain
                   native-type="submit"
                   size="small"
                   @click="handleUserAccountCancel">取消</el-button>
      </div>
    </el-dialog>
    <!--end of add account dialog-->
    <div class="tab-container">
          <template>
            <el-alert type="info"
                      :closable="false"
                      description="为系统定义用户来源，默认支持 LDAP、AD、以及 SSO 集成">
            </el-alert>
          </template>
          <div class="sync-container">
            <el-button v-if="(accounts.length+sso.length) < 2"
                       size="small"
                       type="primary"
                       plain
                       @click="handleUserAccountAdd()">添加</el-button>
          </div>
          <el-table v-if="accounts.length>0"
                    :data="accounts"
                    style="width: 100%;">
            <el-table-column label="账号系统">
              <template slot-scope="scope">
                {{scope.row.type}}
              </template>
            </el-table-column>
            <el-table-column label="主机名">
              <template slot-scope="scope">
                {{scope.row.address}}
              </template>
            </el-table-column>
            <el-table-column label="端口">
              <template slot-scope="scope">
                {{scope.row.port}}
              </template>
            </el-table-column>
            <el-table-column label="DN">
              <template slot-scope="scope">
                {{scope.row.dn}}
              </template>
            </el-table-column>
            <el-table-column label="操作"
                             width="300">
              <template slot-scope="scope">
                <el-button type="primary"
                           size="mini"
                           :loading="syncAccountUserLoading"
                           @click="syncAccountUser()"
                           plain>同步用户数据</el-button>
                <el-button type="primary"
                           size="mini"
                           plain
                           @click="handleUserAccountEdit(scope.row)">编辑</el-button>
                <el-button type="danger"
                           size="mini"
                           @click="handleUserAccountDelete()"
                           plain>删除</el-button>
              </template>
            </el-table-column>
          </el-table>
          <el-table v-if="sso.length>0"
                    :data="sso"
                    style="width: 100%;">
            <el-table-column label="账号系统">
              <template>
                <span>SSO</span>
              </template>
            </el-table-column>
            <el-table-column label="Client Id">
              <template slot-scope="scope">
                {{scope.row.clientId}}
              </template>
            </el-table-column>
            <el-table-column label="Redirect">
              <template slot-scope="scope">
                {{scope.row.redirect}}
              </template>
            </el-table-column>
            <el-table-column label="Secret">
              <template slot-scope="scope">
                {{scope.row.secret}}
              </template>
            </el-table-column>
            <el-table-column label="操作"
                             width="300">
              <template slot-scope="scope">
                <el-button type="primary"
                           size="mini"
                           plain
                           @click="handleSSOEdit(scope.row)">编辑</el-button>
                <el-button type="danger"
                           size="mini"
                           @click="handleSSODelete()"
                           plain>删除</el-button>
              </template>
            </el-table-column>
          </el-table>
    </div>
  </div>
</template>
<script>
import {
  getAccountAPI, deleteAccountAPI, updateAccountAPI, createAccountAPI, syncAccountAPI,
  getSSOAPI, updateSSOAPI, deleteSSOAPI, createSSOAPI
} from '@api'
export default {
  data () {
    return {
      tabPosition: 'top',
      activeTab: '',
      accounts: [],
      sso: [],
      dialogUserAccountAddFormVisible: false,
      dialogUserAccountEditFormVisible: false,
      syncAccountUserLoading: false,
      userAccountAdd: {
        type: '',
        clientId: '',
        userFilter: '(cn=%s)',
        groupFilter: '(cn=*)',
        secret: '',
        redirect: '',
        address: '',
        port: 389,
        isTLS: false,
        dn: '',
        username: '',
        password: ''
      },
      userAccountEdit: {
        type: '',
        clientId: '',
        secret: '',
        redirect: '',
        address: '',
        port: 389,
        isTLS: false,
        dn: '',
        username: '',
        password: ''
      },
      userAccountRules: {
        type: {
          required: true,
          message: '请选择账号类型',
          trigger: ['blur', 'change']
        },
        clientId: {
          required: true,
          message: '请填写 Client Id',
          trigger: ['blur', 'change']
        },
        secret: {
          required: true,
          message: '请填写 Secret',
          trigger: ['blur', 'change']
        },
        redirect: {
          required: true,
          message: '请填写 Redirect 地址',
          trigger: ['blur', 'change']
        },
        address: {
          required: true,
          message: '请填写主机名',
          trigger: ['blur', 'change']
        },
        port: {
          required: true,
          message: '请填写端口',
          trigger: ['blur', 'change']
        },
        username: {
          required: true,
          message: '请填写用户名',
          trigger: ['blur', 'change']
        },
        password: {
          required: true,
          message: '请填写密码',
          trigger: ['blur', 'change']
        },
        dn: {
          required: true,
          message: '请填写 DN',
          trigger: ['blur', 'change']
        },
        userFilter: {
          required: true,
          message: '请填写 userFilter',
          trigger: ['blur', 'change']
        },
        groupFilter: {
          required: true,
          message: '请填写 groupFilter',
          trigger: ['blur', 'change']
        }
      }
    }
  },
  methods: {
    clearValidate (ref) {
      this.$refs[ref].clearValidate()
    },
    handleUserAccountAdd () {
      this.dialogUserAccountAddFormVisible = true
    },
    handleUserAccountEdit (row) {
      this.dialogUserAccountEditFormVisible = true
      this.userAccountEdit = this.$utils.cloneObj(row)
      this.userAccountEdit.password = ''
    },
    handleUserAccountCancel () {
      if (this.$refs.userAccountForm) {
        this.$refs.userAccountForm.resetFields()
        this.dialogUserAccountAddFormVisible = false
      }
      if (this.$refs.userAccountUpdateForm) {
        this.$refs.userAccountUpdateForm.resetFields()
        this.dialogUserAccountEditFormVisible = false
      }
    },
    handleUserAccountDelete () {
      this.$confirm(`确定要删除这个 AD 配置吗？`, '确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        const id = this.currentOrganizationId
        deleteAccountAPI(id).then((res) => {
          this.getAccountConfig()
          this.$message({
            message: 'AD 配置删除成功',
            type: 'success'
          })
        })
      })
    },
    handleSSOEdit (row) {
      this.dialogUserAccountEditFormVisible = true
      this.userAccountEdit = this.$utils.cloneObj(row)
    },
    handleSSODelete (row) {
      this.$confirm(`确定要删除这个 SSO 配置吗？`, '确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        const id = this.currentOrganizationId
        deleteSSOAPI(id).then((res) => {
          this.getAccountConfig()
          this.$message({
            message: 'SSO 配置删除成功',
            type: 'success'
          })
        })
      })
    },
    getAccountConfig () {
      const id = this.currentOrganizationId
      getAccountAPI(id).then((res) => {
        if (!res.resultCode) {
          this.$set(this.accounts, [0], res)
        } else {
          this.$set(this, 'accounts', [])
        }
      })
      getSSOAPI(id).then((res) => {
        if (!res.resultCode) {
          this.$set(this.sso, [0], res)
        } else {
          this.$set(this, 'sso', [])
        }
      })
    },
    syncAccountUser () {
      this.syncAccountUserLoading = true
      const id = this.currentOrganizationId
      syncAccountAPI(id).then((res) => {
        this.syncAccountUserLoading = false
        this.$message({
          message: '用户数据同步成功',
          type: 'success'
        })
      })
    },
    createAccountUser () {
      this.$refs.userAccountForm.validate((valid) => {
        if (valid) {
          const id = this.currentOrganizationId
          if (this.userAccountAdd.type === 'ldap' || this.userAccountAdd.type === 'ad') {
            const payload = this.userAccountAdd
            createAccountAPI(id, payload).then((res) => {
              this.getAccountConfig()
              this.handleUserAccountCancel()
              this.$message({
                message: '用户数据添加成功',
                type: 'success'
              })
            })
          } else if (this.userAccountAdd.type === 'sso') {
            const payload = this.userAccountAdd
            createSSOAPI(id, payload).then((res) => {
              this.getAccountConfig()
              this.handleUserAccountCancel()
              this.$message({
                message: '用户数据添加成功',
                type: 'success'
              })
            })
          }
        } else {
          return false
        }
      })
    },
    updateAccountUser () {
      this.$refs.userAccountUpdateForm.validate((valid) => {
        if (valid) {
          const id = this.currentOrganizationId
          const payload = this.userAccountEdit
          updateAccountAPI(id, payload).then((res) => {
            this.getAccountConfig()
            this.handleUserAccountCancel()
            this.$message({
              message: '用户数据修改成功',
              type: 'success'
            })
          })
        } else {
          return false
        }
      })
    },
    updateSSO () {
      this.$refs.userAccountUpdateForm.validate((valid) => {
        if (valid) {
          const id = this.currentOrganizationId
          const payload = this.userAccountEdit
          updateSSOAPI(id, payload).then((res) => {
            this.getAccountConfig()
            this.dialogUserAccountEditFormVisible = false
            this.$message({
              message: '用户数据修改成功',
              type: 'success'
            })
          })
        } else {
          return false
        }
      })
    }
  },
  computed: {
    currentOrganizationId () {
      return this.$store.state.login.userinfo.organization.id
    }
  },
  activated () {
    this.getAccountConfig()
  }
}
</script>

<style lang="less">
.integration-account-container {
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
      padding-bottom: 0;
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
