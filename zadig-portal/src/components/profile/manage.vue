<template>
  <div v-loading="loading"
       element-loading-text="加载中..."
       element-loading-spinner="iconfont iconfont-loading iconfenzucopy"
       class="setting-profile-container">
    <el-dialog title="修改密码"
               :fullscreen="true"
               class="modifiled-pwd"
               :visible.sync="modifiedPwdDialogVisible"
               center>
      <div class="modifiled-pwd-container">
        <el-form label-position="top"
                 label-width="80px"
                 :rules="rules"
                 ref="ruleForm"
                 :model="pwd">
          <el-form-item label="旧密码"
                        prop="oldPassword">
            <el-input show-password
                      v-model="pwd.oldPassword"></el-input>
          </el-form-item>
          <el-form-item label="新密码"
                        prop="newPassword">
            <el-input show-password
                      v-model="pwd.newPassword"></el-input>
          </el-form-item>
          <el-form-item label="确认新密码"
                        prop="confirmPassword">
            <el-input show-password
                      v-model="pwd.confirmPassword"></el-input>
          </el-form-item>
        </el-form>
      </div>
      <span slot="footer"
            class="dialog-footer">
        <el-button @click="cancelUpdateUserInfo">取 消</el-button>
        <el-button type="primary"
                   @click="updateUserInfo">确 定</el-button>
      </span>
    </el-dialog>
    <div class="section">
      <div class="Box">
        <div class="row">
          <div class="edit-columns">
            <div class="ember-view">
              <div class="avatar-overlay"></div>
              <img src="@assets/icons/others/profile.png"
                   alt=""
                   class="avatar">
            </div>
          </div>
          <div class="info-tag">
            <span class="username"> {{currentEditUserInfo.info.name}} </span>
            <el-tag v-if="currentEditUserInfo.info.isSuperUser"
                    size="small"
                    type="warning">管理员</el-tag>
            <el-tag v-else
                    size="small"
                    type="primary">普通用户</el-tag>
            <el-tag v-if="currentEditUserInfo.info.isTeamLeader"
                    size="small"
                    type="primary">团队管理员</el-tag>

          </div>

          <div class="info-details">
            <table class="table profile-table">
              <tbody>
                <template>
                  <tr>
                    <td>最近登录</td>
                    <td class="">{{$utils.convertTimestamp(currentEditUserInfo.info.lastLogin)}}
                    </td>
                  </tr>
                </template>
                <tr>
                  <td>
                    <span>Kube Config</span>
                    <support-doc :inline="true"
                                 :keyword="{location:'个人中心',key:'KubeConfig'}"></support-doc>
                  </td>
                  <td class="">
                    <el-button @click="downloadPubKey()"
                               type="text">点击下载</el-button>
                  </td>
                </tr>
                <tr v-if="currentEditUserInfo.info.directory ==='system'">
                  <td>
                    <span>修改密码
                    </span>
                  </td>
                  <td>
                    <el-button @click="modifiedPwd"
                               type="text">点击修改</el-button>
                  </td>
                </tr>

                <tr>
                  <td>
                    <span>API Token</span>
                    <support-doc :inline="true"
                                 :keyword="{location:'个人中心',key:'APIToken'}"></support-doc>
                  </td>
                  <td v-if="jwtToken">
                    <span class="token">
                      <el-input size="small"
                                placeholder=""
                                readonly
                                type="text"
                                v-model="jwtToken">
                        <el-button v-clipboard:copy="jwtToken"
                                   v-clipboard:success="copySuccess"
                                   v-clipboard:error="copyError"
                                   slot="append"
                                   icon="el-icon-document-copy">复制</el-button>
                      </el-input>

                    </span>
                  </td>
                  <td v-else>
                    <el-button @click="getToken()"
                               type="text">点击获取</el-button>
                  </td>
                </tr>
                <tr>
                  <td>
                    <span>密钥</span>
                  </td>
                  <td v-if="secret">
                    <span class="token">
                      <el-input size="small"
                                placeholder=""
                                readonly
                                type="text"
                                v-model="secret">
                        <el-button v-clipboard:copy="secret"
                                   v-clipboard:success="copySuccess"
                                   v-clipboard:error="copyError"
                                   slot="append"
                                   icon="el-icon-document-copy">复制</el-button>
                      </el-input>

                    </span>
                  </td>
                  <td v-else>
                    <el-button @click="getToken()"
                               type="text">点击获取</el-button>
                  </td>
                </tr>
                <tr>
                  <td>
                    <span>
                      通知设置
                    </span>
                  </td>
                  <td>
                    <el-checkbox v-model="pipelineNoti.pipelinestatus"
                                 @change="saveSubscribe()"
                                 true-label="*"
                                 false-label="">工作流状态变更</el-checkbox>
                  </td>
                </tr>

              </tbody>
            </table>
          </div>

        </div>
      </div>
    </div>

  </div>
</template>

<script>
import bus from '@utils/event_bus';
import supportDoc from './common/support_doc.vue';
import { getCurrentUserInfoAPI, updateCurrentUserInfoAPI, getJwtTokenAPI, getSubscribeAPI, saveSubscribeAPI, downloadPubKeyAPI, organizationInfoAPI } from '@api';
export default {
  data() {
    let validateNewPass = (rule, value, callback) => {
      if (value === '') {
        callback(new Error('请输入新密码'));
      } else {
        if (this.pwd.confirmPassword !== '') {
          this.$refs['ruleForm'].validateField('confirmPassword');
        }
        callback();
      }
    };
    let validateConfirmPass = (rule, value, callback) => {
      if (value === '') {
        callback(new Error('请再次输入新密码'));
      } else if (value !== this.pwd.newPassword) {
        callback(new Error('两次输入密码不一致!'));
      } else {
        callback();
      }
    };
    return {
      currentEditUserInfo: {
        "info": {
          "id": 5,
          "name": "",
          "email": "",
          "password": "",
          "phone": "",
          "isAdmin": true,
          "isSuperUser": false,
          "isTeamLeader": false,
          "organization_id": 0,
          "directory": "system",
          "teams": []
        },
        "teams": [],
        "organization": {
          "id": 1,
          "name": "",
          "token": "",
          "website": "",
        }
      },
      pwd: {
        oldPassword: '',
        newPassword: '',
        confirmPassword: ''
      },
      jwtToken: null,
      secret: null,
      loading: true,
      modifiedPwdDialogVisible: false,
      sysNoti: {},
      pipelineNoti: {},
      rules: {
        oldPassword: [{ required: true, message: '请输入旧密码', trigger: 'blur' },],
        newPassword: [
          { required: true, validator: validateNewPass, trigger: 'blur' }
        ],
        confirmPassword: [
          { required: true, validator: validateConfirmPass, trigger: 'blur' }
        ],
      }
    };
  },
  methods: {
    getUserInfo() {
      this.loading = true;
      getCurrentUserInfoAPI().then((res) => {
        this.loading = false;
        this.currentEditUserInfo = res;
      });
    },
    getJwtToken() {
      getJwtTokenAPI().then((res) => {
        this.jwtToken = res.token;
      });
    },
    getSecret() {
      const id = 1;
      organizationInfoAPI(id).then((res) => {
        this.secret = res.orgToken;
      });
    },
    copySuccess(event) {
      this.$message({
        message: '已成功复制到剪贴板',
        type: 'success'
      });
    },
    copyError(event) {
      this.$message({
        message: '复制失败',
        type: 'error'
      });
    },
    modifiedPwd() {
      this.modifiedPwdDialogVisible = true;
    },
    updateUserInfo() {
      this.$refs['ruleForm'].validate((valid) => {
        if (valid) {
          const id = this.currentEditUserInfo.info.id;
          const payload = {
            oldPassword: this.pwd.oldPassword,
            newPassword: this.pwd.newPassword
          };
          updateCurrentUserInfoAPI(id, payload).then((res) => {
            this.$message({
              message: '密码修改成功',
              type: 'success'
            });
            this.cancelUpdateUserInfo();
            this.pwd = {
              oldPassword: '',
              newPassword: '',
              confirmPassword: ''
            }
          });
        } else {
          return false;
        }
      });

    },
    cancelUpdateUserInfo() {
      this.$refs['ruleForm'].resetFields();
      this.modifiedPwdDialogVisible = false;
    },
    downloadPubKey() {
      this.$message({
        message: '私钥获取中，请稍候...',
        type: 'info'
      });
      downloadPubKeyAPI().then((res) => {
        this.$message({
          message: '私钥获取完毕，下载后请根据文档使用',
          type: 'success'
        });
        const content = res;
        let fileName = 'config';
        let aTag = document.createElement('a');
        let blob = new Blob([content], { type: 'binary/octet-stream' });
        if (aTag.download !== undefined) {
          aTag.setAttribute('href', window.URL.createObjectURL(blob));
          aTag.setAttribute('download', fileName);
          document.body.appendChild(aTag);
          aTag.click();
          document.body.removeChild(aTag);
        }
      });

    },
    getSubscribe() {
      getSubscribeAPI().then((res) => {
        this.convertData(res);
      });
    },
    saveSubscribe() {
      let payload = this.pipelineNoti;
      payload.type = 2;
      saveSubscribeAPI(payload).then((res) => {
        this.$message({
          message: '通知设置保存成功',
          type: 'success'
        });
        this.getSubscribe();
      });
    },
    convertData(info) {
      if (info.length !== 0) {
        info.forEach(element => {
          if (element.type === 2) {
            this.pipelineNoti = element;
          } else if (element.type === 1) {
            this.sysNoti = element;
          }
        });
      }
    }
  },
  computed: {

  },
  created() {
    bus.$emit(`set-topbar-title`, { title: '用户设置', breadcrumb: [] });
    bus.$emit(`set-sub-sidebar-title`, {
      title: '',
      routerList: []
    });
    this.getUserInfo();
    this.getJwtToken();
    this.getSubscribe();
    this.getSecret();
  },
  components: {
    supportDoc
  }
};
</script>


<style lang="less">
.modifiled-pwd {
  .modifiled-pwd-container {
    width: 300px;
    margin: 0 auto;
  }
}
.setting-profile-container {
  flex: 1;
  position: relative;
  overflow: auto;
  padding: 15px 30px;
  font-size: 13px;
  .module-title h1 {
    font-weight: 200;
    font-size: 2rem;
    margin-bottom: 1.5rem;
  }
  .section {
    margin-bottom: 56px;
    .Box {
      border: 2px solid #f1f1f1;
      border-radius: 3px;
      padding: 55px 20px;
      .username {
        font-weight: 300;
        font-size: 18px;
      }
      .avatar {
        border-radius: 50%;
        width: 50px;
        height: 50px;
        margin-bottom: 10px;
      }
      .edit-columns {
        overflow: hidden;
        .ember-view {
          float: left;
          width: 300px;
        }
        .edit-profile {
          width: 200px;
          margin-top: 15px;
          float: right;
        }
      }
      .info-details {
        h6 {
          font-size: 16px;
          font-weight: 500;
          margin-top: 28px;
          margin-bottom: 10px;
        }
        .profile-table {
          width: 850px;
          margin-top: 15px;
          color: #666f80;

          .token {
            width: 100%;
          }
          tr td {
            padding: 15px 8px;
          }
          tr td:nth-of-type(1) {
            width: 50%;
          }
          tbody > tr > td,
          thead > tr > td {
            border-top: 1px solid #e6e9f0;
            .download-desc {
              cursor: pointer;
              color: #666f80;
              &:hover {
                color: #1989fa;
              }
            }
          }
          tr:first-of-type td {
            border-top: 0;
          }
        }
      }
    }
  }
}
</style>
