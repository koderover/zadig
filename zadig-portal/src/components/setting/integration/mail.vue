<template>
  <div class="intergration-account-container">
    <!--start of edit mail dialog-->
    <el-dialog title="邮件配置-修改"
               :close-on-click-modal="false"
               custom-class="edit-form-dialog"
               :visible.sync="dialogMailEditFormVisible">
      <h3>主机信息</h3>
      <el-form :model="mailHostEdit"
               @submit.native.prevent
               :rules="mailRules"
               ref="mailHostForm">
        <el-form-item label="主机"
                      label-width="80px"
                      prop="name">
          <el-input v-model="mailHostEdit.name"
                    placeholder="主机"
                    autofocus
                    auto-complete="off"></el-input>
        </el-form-item>
        <el-form-item label="端口"
                      label-width="80px"
                      prop="port">
          <el-input-number v-model="mailHostEdit.port"
                           controls-position="right"
                           :min="0"
                           :max="65535"></el-input-number>
        </el-form-item>
        <el-form-item label="用户名"
                      label-width="80px"
                      prop="username">
          <el-input v-model="mailHostEdit.username"
                    placeholder="用户名"
                    autofocus
                    auto-complete="off"></el-input>
        </el-form-item>
        <el-form-item label="密码"
                      label-width="80px"
                      prop="password">
          <el-input v-model="mailHostEdit.password"
                    placeholder="请输入新密码"
                    autofocus
                    auto-complete="off"></el-input>
        </el-form-item>
        <el-form-item label="TLS"
                      label-width="80px"
                      prop="isTLS">
          <el-checkbox v-model="mailHostEdit.isTLS">启用</el-checkbox>
        </el-form-item>
      </el-form>
      <h3>发信设置</h3>
      <el-form :model="mailServiceEdit"
               :rules="mailRules"
               ref="mailServiceForm">
        <el-form-item label="名称"
                      label-width="80px"
                      prop="name">
          <el-input v-model="mailServiceEdit.name"
                    placeholder="名称"
                    autofocus
                    auto-complete="off"></el-input>
        </el-form-item>
        <el-form-item label="发信地址"
                      label-width="80px"
                      prop="address">
          <el-input v-model="mailServiceEdit.address"
                    placeholder="发信地址"
                    autofocus
                    auto-complete="off"></el-input>
        </el-form-item>
        <el-form-item label="显示名称"
                      label-width="80px"
                      prop="displayName">
          <el-input v-model="mailServiceEdit.displayName"
                    placeholder="显示名称"
                    autofocus
                    auto-complete="off"></el-input>
        </el-form-item>
        <el-form-item label="主题前缀"
                      label-width="80px"
                      prop="theme">
          <el-input v-model="mailServiceEdit.theme"
                    placeholder="主题前缀"
                    autofocus
                    auto-complete="off"></el-input>
        </el-form-item>

      </el-form>
      <div slot="footer"
           class="dialog-footer">
        <el-button type="primary"
                   native-type="submit"
                   size="small"
                   @click="updateMailConfig()"
                   class="start-create">确定</el-button>
        <el-button plain
                   native-type="submit"
                   size="small"
                   @click="handleMailCancel()">取消</el-button>
      </div>
    </el-dialog>
    <!--end of edit mail dialog-->

    <!--start of add mail dialog-->
    <el-dialog title="邮件配置-新增"
               :close-on-click-modal="false"
               custom-class="edit-form-dialog"
               :visible.sync="dialogMailAddFormVisible">
      <h3>主机信息</h3>
      <el-form :model="mailHostAdd"
               @submit.native.prevent
               :rules="mailRules"
               ref="mailHostForm">
        <el-form-item label="主机"
                      label-width="80px"
                      prop="name">
          <el-input v-model="mailHostAdd.name"
                    placeholder="主机"
                    autofocus
                    auto-complete="off"></el-input>
        </el-form-item>
        <el-form-item label="端口"
                      label-width="80px"
                      prop="port">
          <el-input-number v-model="mailHostAdd.port"
                           controls-position="right"
                           :min="0"
                           :max="65535"></el-input-number>
        </el-form-item>
        <el-form-item label="用户名"
                      label-width="80px"
                      prop="username">
          <el-input v-model="mailHostAdd.username"
                    placeholder="用户名"
                    autofocus
                    auto-complete="off"></el-input>
        </el-form-item>
        <el-form-item label="密码"
                      label-width="80px"
                      prop="password">
          <el-input v-model="mailHostAdd.password"
                    placeholder="密码"
                    autofocus
                    auto-complete="off"></el-input>
        </el-form-item>
        <el-form-item label="TLS"
                      label-width="80px"
                      prop="isTLS">
          <el-checkbox v-model="mailHostAdd.isTLS">启用</el-checkbox>
        </el-form-item>
      </el-form>
      <h3>发信设置</h3>
      <el-form :model="mailServiceAdd"
               :rules="mailRules"
               ref="mailServiceForm">
        <el-form-item label="名称"
                      label-width="80px"
                      prop="name">
          <el-input v-model="mailServiceAdd.name"
                    placeholder="名称"
                    autofocus
                    auto-complete="off"></el-input>
        </el-form-item>
        <el-form-item label="发信地址"
                      label-width="80px"
                      prop="address">
          <el-input v-model="mailServiceAdd.address"
                    placeholder="发信地址"
                    autofocus
                    auto-complete="off"></el-input>
        </el-form-item>
        <el-form-item label="显示名称"
                      label-width="80px"
                      prop="displayName">
          <el-input v-model="mailServiceAdd.displayName"
                    placeholder="显示名称"
                    autofocus
                    auto-complete="off"></el-input>
        </el-form-item>
        <el-form-item label="主题前缀"
                      label-width="80px"
                      prop="theme">
          <el-input v-model="mailServiceAdd.theme"
                    placeholder="主题前缀"
                    autofocus
                    auto-complete="off"></el-input>
        </el-form-item>

      </el-form>
      <div slot="footer"
           class="dialog-footer">
        <el-button type="primary"
                   native-type="submit"
                   size="small"
                   @click="createMailConfig()"
                   class="start-create">确定</el-button>
        <el-button plain
                   native-type="submit"
                   size="small"
                   @click="handleMailCancel()">取消</el-button>
      </div>
    </el-dialog>
    <!--end of add mail dialog-->
          <template>
            <el-alert type="info"
                      :closable="false"
                      description="为系统定义邮件集成，用于系统内部的发信设置">
            </el-alert>
          </template>
          <div class="sync-container">
            <el-button v-if="mailHosts.length === 0"
                       size="small"
                       type="primary"
                       plain
                       @click="handleMailAdd">添加</el-button>
          </div>
          <el-table :data="mailHosts"
                    style="width: 100%">
            <el-table-column label="主机">
              <template slot-scope="scope">
                {{scope.row.name}}
              </template>
            </el-table-column>
            <el-table-column label="端口">
              <template slot-scope="scope">
                {{scope.row.port}}
              </template>
            </el-table-column>
            <el-table-column label="用户名">
              <template slot-scope="scope">
                {{scope.row.username}}
              </template>
            </el-table-column>
            <el-table-column label="开启 TLS">
              <template slot-scope="scope">
                {{scope.row.isTLS?'是':'否'}}
              </template>
            </el-table-column>
            <el-table-column label="操作"
                             width="160">
              <template slot-scope="scope">
                <el-button type="primary"
                           size="mini"
                           plain
                           @click="handleMailEdit">编辑</el-button>
                <el-button type="danger"
                           size="mini"
                           @click="handleMailDelete"
                           plain>删除</el-button>
              </template>
            </el-table-column>
          </el-table>
     </div>
</template>
<script>
import { getEmailHostAPI, deleteEmailHostAPI, createEmailHostAPI, getEmailServiceAPI, deleteEmailServiceAPI, createEmailServiceAPI } from '@api';
export default {
  data() {
    return {
      mailHosts: [],
      mailService: {},
      mailHostAdd: {
        "name": "",
        "port": 465,
        "username": "",
        "password": "",
        "isTLS": false
      },
      mailHostEdit: {
        "name": "",
        "port": 465,
        "username": "",
        "password": "",
        "isTLS": false
      },
      mailServiceAdd: {
        "name": "",
        "address": "",
        "displayName": "",
        "theme": ""
      },
      mailServiceEdit: {
        "name": "string",
        "address": "string",
        "displayName": "string",
        "theme": "string"
      },
      mailRules: {
        name: {
          required: true,
          message: '请填写主机名',
          trigger: ['blur', 'change']
        },
        address: {
          required: true,
          message: '请填写发信地址',
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
        theme: {
          required: true,
          message: '请填写主题',
          trigger: ['blur', 'change']
        },
        displayName: {
          required: true,
          message: '请填写显示名称',
          trigger: ['blur', 'change']
        }
      },
      dialogMailAddFormVisible: false,
      dialogMailEditFormVisible: false,
    };
  },
  methods: {
    clearValidate(ref) {
      this.$refs[ref].clearValidate();
    },
    handleMailAdd() {
      this.dialogMailAddFormVisible = true;
    },
    handleMailEdit(row) {
      this.dialogMailEditFormVisible = true;
      this.mailHostEdit = this.$utils.cloneObj(this.mailHosts[0]);
      this.mailServiceEdit = this.$utils.cloneObj(this.mailService);
    },
    handleMailCancel() {
      this.dialogMailAddFormVisible = false;
      this.dialogMailEditFormVisible = false;
      this.$refs['mailHostForm'].resetFields();
      this.$refs['mailServiceForm'].resetFields();
    },
    handleMailDelete() {
      const id = this.currentOrganizationId;
      this.$confirm(`确定要删除这个邮件配置吗？`, '确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        Promise.all([deleteEmailHostAPI(id), deleteEmailServiceAPI(id)]).then(
          (res) => {
            this.mailService = {};
            this.mailHosts = [];
            this.getMailHostConfig();
            this.getMailServiceConfig();
            this.$message({
              message: '主机配置删除成功',
              type: 'success'
            });
          }
        ).catch(
          (err) => {
          }
        )

      });
    },
    getMailHostConfig() {
      const id = this.currentOrganizationId;
      getEmailHostAPI(id).then((res) => {
        if (!res.resultCode) {
          this.$set(this.mailHosts, [0], res);
        } else {
          this.$set(this, 'mailHosts', []);
        }
      })
    },
    getMailServiceConfig() {
      const id = this.currentOrganizationId;
      getEmailServiceAPI(id).then((res) => {
        this.mailService = res;
      })
    },
    createMailConfig() {
      const refs = [this.$refs['mailHostForm'], this.$refs['mailServiceForm']];
      const payload1 = this.mailHostAdd;
      const payload2 = this.mailServiceAdd;
      const id = this.currentOrganizationId;
      Promise.all(refs.map(r => r.validate())).then(() => {
        Promise.all([createEmailHostAPI(id, payload1), createEmailServiceAPI(id, payload2)]).then(
          (res) => {
            this.getMailHostConfig();
            this.getMailServiceConfig();
            this.handleMailCancel();
            this.$message({
              message: '主机配置新增成功',
              type: 'success'
            });
          }
        ).catch(
          (err) => {
          }
        )
      });
    },
    updateMailConfig() {
      const refs = [this.$refs['mailHostForm'], this.$refs['mailServiceForm']];
      const payload1 = this.mailHostEdit;
      const payload2 = this.mailServiceEdit;
      const id = this.currentOrganizationId;
      Promise.all(refs.map(r => r.validate())).then(() => {
        Promise.all([createEmailHostAPI(id, payload1), createEmailServiceAPI(id, payload2)]).then(
          (res) => {
            this.getMailHostConfig();
            this.getMailServiceConfig();
            this.handleMailCancel();
            this.$message({
              message: '主机配置修改成功',
              type: 'success'
            });
          }
        ).catch(
          (err) => {
          }
        )
      });
    }
  },
  computed: {
    currentOrganizationId() {
      return this.$store.state.login.userinfo.organization.id;
    }
  },
  activated() {
    this.getMailHostConfig();
    this.getMailServiceConfig();
  }
}
</script>

<style lang="less">
.intergration-account-container {
  flex: 1;
  position: relative;
  overflow: auto;
  font-size: 13px;
  .module-title h1 {
    font-weight: 200;
    font-size: 2rem;
    margin-bottom: 1.5rem;
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
    margin-top: 15px;
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
      text-align: center;
      border-bottom: 1px solid #e4e4e4;
      padding: 15px;
      .el-dialog__close {
        font-size: 10px;
      }
    }
    .el-dialog__body {
      padding-bottom: 0px;
    }
    .el-dialog__body {
      padding: 0px 20px;
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