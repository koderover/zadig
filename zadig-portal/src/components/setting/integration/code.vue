<template>
  <div class="intergration-code-container">
    <!--start of edit code dialog-->
    <el-dialog title="代码管理-编辑"
               custom-class="edit-form-dialog"
               :close-on-click-modal="false"
               :visible.sync="dialogCodeEditFormVisible">
      <template>
        <el-alert type="info"
                  :closable="false">
          <slot>{{`应用授权的回调地址请按照下面填写，应用权限请勾选:api、read_user、read_repository，其它配置可点击`}} <el-link
                     style="font-size:14px"
                     type="primary"
                     :href="`https://docs.koderover.com/zadig/settings/codehost/`"
                     :underline="false"
                     target="_blank">帮助</el-link> 查看配置样例</slot>
        </el-alert>
      </template>
      <div class="highlighter-rouge">
        <div class="highlight">
          <span class="code-line">
            {{`${$utils.getOrigin()}/api/directory/codehosts/callback`}}
            <span v-clipboard:copy="`${$utils.getOrigin()}/api/directory/codehosts/callback`"
                  v-clipboard:success="copyCommandSuccess"
                  v-clipboard:error="copyCommandError"
                  class="el-icon-document-copy copy"></span>
          </span>
        </div>
      </div>
      <el-form :model="codeEdit"
               :rules="codeRules"
               ref="codeUpdateForm">
        <el-form-item label="代码源"
                      prop="type">
          <el-select v-model="codeEdit.type"
                     disabled>
            <el-option label="Gitlab"
                       value="gitlab"></el-option>
            <el-option label="GitHub"
                       value="github"></el-option>
            <el-option label="Gerrit"
                       value="gerrit"></el-option>
          </el-select>
        </el-form-item>
        <template v-if="codeEdit.type==='gitlab'||codeEdit.type==='github'">
          <el-form-item v-if="codeEdit.type==='gitlab'"
                        label="GitLab 服务 URL"
                        prop="address">
            <el-input v-model="codeEdit.address"
                      placeholder="GitLab 服务 URL"
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item :label="codeEdit.type==='gitlab'?'Application ID':'Client ID'"
                        prop="applicationId">
            <el-input v-model="codeEdit.applicationId"
                      :placeholder="codeEdit.type==='gitlab'?'Application ID':'Client ID'"
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item :label="codeEdit.type==='gitlab'?'Secret':'Client Secret'"
                        prop="clientSecret">
            <el-input v-model="codeEdit.clientSecret"
                      :placeholder="codeEdit.type==='gitlab'?'Secret':'Client Secret'"
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item v-if="codeEdit.type==='github'"
                        label="Organization"
                        prop="namespace">
            <el-input v-model="codeEdit.namespace"
                      placeholder="Organization"
                      auto-complete="off"></el-input>
          </el-form-item>
        </template>
        <template v-else-if="codeEdit.type==='gerrit'">
          <el-form-item label="Gerrit 服务 URL"
                        prop="address">
            <el-input v-model="codeEdit.address"
                      placeholder="Gerrit 服务 URL"
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item label="用户名"
                        prop="username">
            <el-input v-model="codeEdit.username"
                      placeholder="Username"
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item label="密码"
                        prop="password">
            <el-input v-model="codeEdit.password"
                      placeholder="Password"
                      auto-complete="off"></el-input>
          </el-form-item>
        </template>

      </el-form>
      <div slot="footer"
           class="dialog-footer">
        <el-button type="primary"
                   native-type="submit"
                   size="small"
                   class="start-create"
                   @click="updateCodeConfig">{{codeEdit.type==='gerrit'?'确定':'前往授权'}}</el-button>
        <el-button plain
                   native-type="submit"
                   size="small"
                   @click="dialogCodeEditFormVisible = false">取消</el-button>
      </div>
    </el-dialog>
    <!--end of edit code dialog-->

    <!--start of add code dialog-->
    <el-dialog title="代码管理-添加"
               custom-class="edit-form-dialog"
               :close-on-click-modal="false"
               :visible.sync="dialogCodeAddFormVisible">
      <template>
        <el-alert type="info"
                  :closable="false">
          <slot>
            <span class="tips">{{`- 应用授权的回调地址请填写:`}}</span>
            <span class="tips code-line">
              {{`${$utils.getOrigin()}/api/directory/codehosts/callback`}}
              <span v-clipboard:copy="`${$utils.getOrigin()}/api/directory/codehosts/callback`"
                    v-clipboard:success="copyCommandSuccess"
                    v-clipboard:error="copyCommandError"
                    class="el-icon-document-copy copy"></span>
            </span>
            <span class="tips">- 应用权限请勾选：api、read_user、read_repository</span>
            <span class="tips">- 其它配置可以点击
              <el-link style="font-size:13px"
                       type="primary"
                       :href="`https://docs.koderover.com/zadig/settings/codehost/`"
                       :underline="false"
                       target="_blank">帮助</el-link> 查看配置样例
            </span>
          </slot>
        </el-alert>
      </template>
      <el-form :model="codeAdd"
               :rules="codeRules"
               ref="codeForm">
        <el-form-item label="代码源"
                      prop="type">
          <el-select v-model="codeAdd.type">
            <el-option label="GitLab"
                       value="gitlab"></el-option>
            <el-option label="GitHub"
                       value="github"></el-option>
            <el-option label="Gerrit"
                       value="gerrit"></el-option>
          </el-select>
        </el-form-item>
        <template v-if="codeAdd.type==='gitlab'||codeAdd.type==='github'">
          <el-form-item v-if="codeAdd.type==='gitlab'"
                        label="GitLab 服务 URL"
                        prop="address">
            <el-input v-model="codeAdd.address"
                      placeholder="GitLab 服务 URL"
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item :label="codeAdd.type==='gitlab'?'Application ID':'Client ID'"
                        prop="applicationId">
            <el-input v-model="codeAdd.applicationId"
                      :placeholder="codeAdd.type==='gitlab'?'Application ID':'Client ID'"
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item :label="codeAdd.type==='gitlab'?'Secret':'Client Secret'"
                        prop="clientSecret">
            <el-input v-model="codeAdd.clientSecret"
                      :placeholder="codeAdd.type==='gitlab'?'Secret':'Client Secret'"
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item v-if="codeAdd.type==='github'"
                        label="Organization"
                        prop="namespace">
            <el-input v-model="codeAdd.namespace"
                      placeholder="Organization"
                      auto-complete="off"></el-input>
          </el-form-item>
        </template>
        <template v-else-if="codeAdd.type==='gerrit'">
          <el-form-item label="Gerrit 服务 URL"
                        prop="address">
            <el-input v-model="codeAdd.address"
                      placeholder="Gerrit 服务 URL"
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item label="用户名"
                        prop="username">
            <el-input v-model="codeAdd.username"
                      placeholder="Username"
                      auto-complete="off"></el-input>
          </el-form-item>
          <el-form-item label="密码"
                        prop="password">
            <el-input v-model="codeAdd.password"
                      placeholder="Password"
                      auto-complete="off"></el-input>
          </el-form-item>
        </template>
      </el-form>
      <div slot="footer"
           class="dialog-footer">
        <el-button type="primary"
                   native-type="submit"
                   size="small"
                   class="start-create"
                   @click="createCodeConfig">{{codeAdd.type==='gerrit'?'确定':'前往授权'}}</el-button>

        <el-button plain
                   native-type="submit"
                   size="small"
                   @click="dialogCodeAddFormVisible = false">取消</el-button>
      </div>
    </el-dialog>
    <!--end of add code dialog-->
          <div>
            <template>
              <el-alert type="info"
                        :closable="false"
                        description="为系统定义代码源，默认支持 GitHub、GitLab 集成">
              </el-alert>
            </template>
            <div class="sync-container">
              <el-button type="primary"
                         size="small"
                         @click="handleCodeAdd"
                         plain>添加</el-button>
              <span class="switch-span"
                    :style="{color: proxyInfo.enable_repo_proxy?'#409EFF':'#303133'}">启用代理</span>
              <el-switch size="small"
                         :value="proxyInfo.enable_repo_proxy"
                         @change="changeProxy"></el-switch>
            </div>
            <el-table :data="code"
                      style="width: 100%">
              <el-table-column label="代码源">
                <template slot-scope="scope">
                  <span
                        v-if="scope.row.type==='gitlab'||scope.row.type==='gerrit'">{{scope.row.type}}</span>
                  <span
                        v-if="scope.row.type==='github'">{{scope.row.type}}({{scope.row.namespace}})</span>
                </template>
              </el-table-column>
              <el-table-column label="URL">
                <template slot-scope="scope">
                  {{scope.row.address}}
                </template>
              </el-table-column>
              <el-table-column label="授权信息">
                <template slot-scope="scope">
                  <span
                        :class="scope.row.ready?'text-success':'text-failed'">{{scope.row.ready?'授权成功':'授权失败'}}</span>
                </template>
              </el-table-column>
              <el-table-column label="最后更新">
                <template slot-scope="scope">
                  {{$utils.convertTimestamp(scope.row.updated_at)}}
                </template>
              </el-table-column>
              <el-table-column label="操作"
                               width="160">
                <template slot-scope="scope">
                  <el-button type="primary"
                             size="mini"
                             plain
                             @click="handleCodeEdit(scope.row)">编辑</el-button>
                  <el-button type="danger"
                             size="mini"
                             @click="handleCodeDelete(scope.row)"
                             plain>删除</el-button>
                </template>
              </el-table-column>
            </el-table>
          </div>
  </div>
</template>
<script>
import {
  getCodeSourceByAdminAPI, deleteCodeSourceAPI, updateCodeSourceAPI, createCodeSourceAPI, getProxyConfigAPI, updateProxyConfigAPI
} from '@api';
let validateGitURL = (rule, value, callback) => {
  if (value === '') {
    callback(new Error('请输入服务 URL'));
  } else {
    if (value.endsWith('/')) {
      callback(new Error('URL 末尾不能包含 /'));
    } else {
      callback();
    }
  }
};
export default {
  data() {
    return {
      proxyInfo: {
        id: '',
        type: '',
        address: '',
        port: undefined,
        username: '',
        password: '',
        enable_repo_proxy: false,
        enable_application_proxy: false,
      },
      code: [],
      dialogCodeAddFormVisible: false,
      dialogCodeEditFormVisible: false,
      codeEdit: {
        "name": "",
        "type": "",
        "address": "",
        "accessToken": "",
        "applicationId": "",
        "clientSecret": "",
      },
      codeAdd: {
        "name": "",
        "namespace": "",
        "type": "gitlab",
        "address": "",
        "accessToken": "",
        "applicationId": "",
        "clientSecret": "",
      },
      codeRules: {
        type: {
          required: true,
          message: '请选择代码源类型',
          trigger: ['blur']
        },
        address: [{
          type: 'url',
          message: '请输入正确的 URL，包含协议',
          trigger: ['blur', 'change']
        }, {
          required: true,
          trigger: 'change',
          validator: validateGitURL,
        }],
        accessToken: {
          required: true,
          message: '请填写 Access Token',
          trigger: ['blur']
        },
        applicationId: {
          required: true,
          message: '请填写 Id',
          trigger: ['blur']
        },
        clientSecret: {
          required: true,
          message: '请填写 Secret',
          trigger: ['blur']
        },
        namespace: {
          required: true,
          message: '请填写 Org',
          trigger: ['blur']
        },
        username: {
          required: true,
          message: '请填写 Username',
          trigger: ['blur']
        },
        password: {
          required: true,
          message: '请填写 Password',
          trigger: ['blur']
        }
      },
    };
  },
  methods: {
    handleCodeAdd() {
      this.dialogCodeAddFormVisible = true;
    },
    handleCodeDelete(row) {
      this.$confirm(`确定要删除这个代码源吗？`, '确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        deleteCodeSourceAPI(row.id).then((res) => {
          this.getCodeConfig();
          this.$message({
            message: '代码源删除成功',
            type: 'success'
          });
        });
      });
    },
    handleCodeEdit(row) {
      this.dialogCodeEditFormVisible = true;
      this.codeEdit = this.$utils.cloneObj(row);
    },
    handleCodeCancel() {
      if (this.$refs['codeForm']) {
        this.$refs['codeForm'].resetFields();
        this.dialogCodeAddFormVisible = false;
      }
      else if (this.$refs['codeUpdateForm']) {
        this.$refs['codeUpdateForm'].resetFields();
        this.dialogCodeEditFormVisible = false;
      }
    },
    clearValidate(ref) {
      this.$refs[ref].clearValidate();
    },
    createCodeConfig() {
      this.$refs['codeForm'].validate((valid) => {
        if (valid) {
          const id = this.currentOrganizationId;
          const payload = this.codeAdd;
          const redirect_url = window.location.href.split('?')[0];
          const provider = this.codeAdd.type;
          createCodeSourceAPI(id, payload).then((res) => {
            const code_source_id = res.id;
            this.getCodeConfig();
            this.$message({
              message: '代码源添加成功',
              type: 'success'
            });
            if (payload.type === 'gitlab' || payload.type === 'github') {
              this.goToCodeHostAuth(code_source_id, redirect_url, provider);
            }
            this.handleCodeCancel();
          });

        } else {
          return false;
        }
      });
    },
    updateCodeConfig() {
      this.$refs['codeUpdateForm'].validate((valid) => {
        if (valid) {
          const id = this.currentOrganizationId;
          const payload = this.codeEdit;
          const code_source_id = this.codeEdit.id;
          const redirect_url = window.location.href.split('?')[0];
          const provider = this.codeEdit.type;
          updateCodeSourceAPI(code_source_id, payload).then((res) => {
            this.getCodeConfig();
            if (payload.type === 'gitlab' || payload.type === 'github') {
              this.$message({
                message: '代码源修改成功，正在前往授权',
                type: 'success'
              });
              this.goToCodeHostAuth(code_source_id, redirect_url, provider);
            }
            else {
              this.handleCodeCancel()
              this.$message({
                message: '代码源修改成功',
                type: 'success'
              });
            }

          });

        } else {
          return false;
        }
      });
    },
    getCodeConfig() {
      const id = this.currentOrganizationId;
      getCodeSourceByAdminAPI(id).then((res) => {
        this.code = res;
      });
    },
    goToCodeHostAuth(code_source_id, redirect_url, provider) {
      window.location.href = `/api/directory/codehostss/${code_source_id}/auth?redirect=${redirect_url}&provider=${provider}`;
    },
    copyCommandSuccess(event) {
      this.$message({
        message: '地址已成功复制到剪贴板',
        type: 'success'
      });
    },
    copyCommandError(event) {
      this.$message({
        message: '地址复制失败',
        type: 'error'
      });
    },
    changeProxy(value) {
      if (!this.proxyInfo.id || this.proxyInfo.type === 'no') {
        this.proxyInfo.enable_repo_proxy = false;
        this.$message.error('未配置代理，请先前往「系统配置」-「代理配置」配置代理！')
        return;
      }
      this.proxyInfo.enable_repo_proxy = value;
      updateProxyConfigAPI(this.proxyInfo.id, this.proxyInfo).then(response => {
        if (response.message === 'success') {
          var mess = value ? '启用代理成功！' : '成功关闭代理！';
          this.$message({
            message: `${mess}`,
            type: 'success'
          })
        } else {
          this.$message.error(response.message);
        }
      }).catch(err => {
        this.proxyInfo.enable_repo_proxy = !value;
        this.$message.error(`修改配置失败：${err}`);
      })
    },
    getProxyConfig() {
      getProxyConfigAPI().then(response => {
        if (response.length > 0) {
          this.proxyInfo = Object.assign({}, this.proxyInfo, response[0]);
        }
      }).catch(error => {
        this.$message.error(`获取代理配置失败：${error}`);
      })
    }
  },
  computed: {
    currentOrganizationId() {
      return this.$store.state.login.userinfo.organization.id;
    }
  },
  watch: {
    'codeAdd.type'(val) {
      this.$refs['codeForm'].clearValidate();
    }
  },
  activated() {
    this.getProxyConfig();
    this.getCodeConfig();
  }
}
</script>

<style lang="less">
.intergration-code-container {
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
      .switch-span {
        display: inline-block;
        height: 20px;
        line-height: 20px;
        font-size: 14px;
        font-weight: 500;
        vertical-align: middle;
        margin-left: 10px;
        margin-right: 5px;
        transition: color 0.5s;
      }
    }
  }
  .text-success {
    color: rgb(82, 196, 26);
  }
  .text-failed {
    color: #ff1949;
  }
  .edit-form-dialog {
    width: 580px;
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
    .tips {
      display: block;
      &.code-line {
        border-radius: 2px;
        font-family: monospace, monospace;
        display: inline-block;
        font-size: 14px;
        color: #ecf0f1;
        word-break: break-all;
        word-wrap: break-word;
        background-color: #334851;
        padding-left: 10px;
        .copy {
          font-size: 16px;
          cursor: pointer;
          &:hover {
            color: #13ce66;
          }
        }
      }
    }
  }
}
</style>