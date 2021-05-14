<template >
  <div class="setup-admin">
    <div class="main-container">
      <div class="brand-wrapper">
        <span class="name">Zadig</span>
      </div>
      <p class="description">系统初始化设置</p>
      <el-form :model="organizationInfos"
               :rules="rules"
               ref="infoForm"
               label-width="100px"
               label-position="top">
        <el-form-item prop="adminName">
          <el-input v-model="organizationInfos.adminName"
                    placeholder="管理员用户名">
          </el-input>
        </el-form-item>
        <el-form-item prop="adminEmail">
          <el-input v-model="organizationInfos.adminEmail"
                    placeholder="管理员邮箱">
          </el-input>
        </el-form-item>
        <el-form-item prop="adminPassword">
          <el-input v-model="organizationInfos.adminPassword"
                    type="password"
                    autocomplete="off"
                    show-password
                    placeholder="密码">
          </el-input>
        </el-form-item>
        <el-form-item prop="checkPass">
          <el-input v-model="organizationInfos.checkPass"
                    type="password"
                    autocomplete="off"
                    show-password
                    placeholder="确认密码">
          </el-input>
        </el-form-item>
      </el-form>
      <div class="operation">
        <el-button size="medium"
                   class="btn"
                   @click="saveChange('infoForm')"
                   @keyup.enter.native="saveChange('infoForm')"
                   type="primary"
                   round>确定</el-button>
      </div>

    </div>
  </div>
</template>

<script>
import { createOrganizationInfoAPI, installationAnalysisRequestAPI, userLoginAPI, getCurrentUserInfoAPI } from '@api';
export default {
  data() {
    let validatePass = (rule, value, callback) => {
      if (value === '') {
        callback(new Error('请再次输入密码'));
      } else if (value !== this.organizationInfos.adminPassword) {
        callback(new Error('两次输入密码不一致!'));
      } else {
        callback();
      }
    };
    return {
      organizationInfos: {
        "adminName": "",
        "adminEmail": "",
        "adminPhone": "",
        "adminPassword": "",
        "checkPass": ""
      },
      rules: {
        adminName: [{
          required: true,
          message: '请输入管理员用户名',
          trigger: ['blur', 'change']
        }],
        adminPassword: [{
          required: true,
          message: '请输入管理员密码',
          trigger: ['blur', 'change']
        }],
        adminPhone: [{
          required: true,
          message: '请输入管理员手机号',
          trigger: ['blur', 'change']
        }],
        adminEmail: [{
          required: true,
          message: '请输入邮箱',
          trigger: ['blur', 'change']
        },
        {
          type: 'email',
          message: '请输入正确的邮箱地址',
          trigger: ['blur', 'change']
        }
        ],
        checkPass: [
          { validator: validatePass, trigger: ['blur', 'change'] }
        ],
      }
    }
  },
  methods: {
    saveChange(formName) {
      this.$refs[formName].validate((valid) => {
        if (valid) {
          const orgPayload = this.organizationInfos;
          const loginPayload = {
            email: this.organizationInfos.adminEmail,
            password: this.organizationInfos.adminPassword
          };
          const publicKey = `-----BEGIN RSA PUBLIC KEY-----
    MIIBpTANBgkqhkiG9w0BAQEFAAOCAZIAMIIBjQKCAYQAz5IqagSbovHGXmUf7wTB
    XrR+DZ0u3p5jsgJW08ISJl83t0rCCGMEtcsRXJU8bE2dIIfndNwvmBiqh13/WnJd
    +jgyIm6i1ZfNmf/R8pEqVXpOAOuyoD3VLT9tfWvz9nPQbjVI+PsUHH7nVR0Jwxem
    NsH/7MC2O15t+2DVC1533UlhjT/pKFDdTri0mgDrLZHp6gPF5d7/yQ7cPbzv6/0p
    0UgIdStT7IhkDfsJDRmLAz09znv5tQQtHfJIMdAKxwHw9mExcL2gE40sOezrgj7m
    srOnJd65N8anoMGxQqNv+ycAHB9aI1Yrtgue2KKzpI/Fneghd/ZavGVFWKDYoFP3
    531Ga/CiCwtKfM0vQezfLZKAo3qpb0Edy2BcDHhLwidmaFwh8ZlXuaHbNaF/FiVR
    h7uu0/9B/gn81o2f+c8GSplWB5bALhQH8tJZnvmWZGI9OnrIlWmQZsuUBooTul9Q
    ZJ/w3sE1Zoxa+Or1/eWijqtIfhukOJBNyGaj+esFg6uEeBgHAgMBAAE=
    -----END RSA PUBLIC KEY-----`;
          let encryptor = new JSEncrypt();
          encryptor.setPublicKey(publicKey);
          const installPayload = {
            data: encryptor.encrypt(JSON.stringify({
              domain: window.location.hostname,
              username: orgPayload.adminName,
              email: orgPayload.adminEmail,
              created_at: Math.floor(Date.now() / 1000),
            }))
          };
          createOrganizationInfoAPI(orgPayload).then((res) => {
            this.$message({
              message: '添加成功',
              type: 'success'
            });
          }).then(() => {
            const orgId = 1;
            userLoginAPI(orgId, loginPayload).then((res) => {
              this.$store.commit('INJECT_PROFILE', res);
              this.$router.push('/v1/projects/create');
            })
            installationAnalysisRequestAPI(installPayload)
              .then((res) => { })
          })
        } else {
          return false;
        }
      });
    },
    checkInstalled() {
      getCurrentUserInfoAPI().then(
        response => {
          this.$router.push('/v1/status');
        }
      );
    }
  },
  created() {
    // this.checkInstalled();
  }
}
</script>
<style lang="less">
.setup-admin {
  display: flex;
  justify-content: center;
  align-items: center;
  width: 500px;
  margin: 0 auto;
  .main-container {
    width: 500px;
    padding: 40px;
    padding-right: 60px;
    border-radius: 4px;
    background: #fff;
    border: 0;
    border-radius: 27.5px;
    box-shadow: 0 10px 30px 0 #dcdfe6;
    .brand-wrapper {
      margin-bottom: 19px;
      .name {
        font-size: 30px;
        display: inline-block;
        font-weight: 500;
        font-family: "Helvetica Neue";
        background: linear-gradient(25deg, #ff6302 0%, #ff2968 100%);
        display: inline-block;
        -webkit-background-clip: text;
        -webkit-text-fill-color: transparent;
      }
    }
    .description {
      font-size: 20px;
      color: #000;
      font-weight: normal;
      margin-bottom: 23px;
    }
  }
  .operation {
    margin-top: 16px;
    padding: 10px 55px;
    display: flex;
    justify-content: center;
    .btn {
      width: 100%;
    }
  }
}
</style>
