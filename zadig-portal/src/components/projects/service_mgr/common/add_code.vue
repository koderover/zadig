<template>
  <div class="add-code-container">
    <el-alert title="检测到代码源尚未集成请先集成代码源后再进行添加构建操作"
              :closable="false"
              type="warning">
    </el-alert>
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
    <el-form :model="codeAdd"
             :rules="codeRules"
             status-icon
             label-position="top"
             ref="codeForm">
      <el-form-item label="代码源"
                    prop="type">
        <el-select style="width:100%"
                   v-model="codeAdd.type">
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
                 @click="cancel">取消</el-button>
    </div>
  </div>
</template>
<script>
import {
  createCodeSourceAPI
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
    clearValidate(ref) {
      this.$refs[ref].clearValidate();
    },
    cancel() {
      this.$emit('cancel', true);
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
}
</script>
<style lang="less">
.add-code-container {
  padding: 10px 15px;
  font-size: 13px;
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
</style>
