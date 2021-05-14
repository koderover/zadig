<template>
  <div class="login tab-box">
    <div class="container-fluid">
      <div class="row">
        <div class="col-lg-7 col-md-12 col-pad-0 form-section">
          <div class="login-inner-form">
            <div class="details">
              <header>
                <span class="name">Zadig</span>
                <h3>登入账户</h3>
              </header>
              <section>
                <el-form :model="loginForm"
                         status-icon
                         :rules="rules"
                         ref="loginForm">
                  <el-form-item label=""
                                prop="username">
                    <el-input v-model="loginForm.username"
                              placeholder="邮箱"
                              autocomplete="off"></el-input>
                  </el-form-item>
                  <el-form-item label=""
                                prop="password">
                    <el-input type="password"
                              @keyup.enter.native="login"
                              v-model="loginForm.password"
                              autocomplete="off"
                              placeholder="密码"
                              show-password></el-input>
                  </el-form-item>

                </el-form>
                <el-button type="submit"
                           @click="login"
                           class="btn-md btn-theme btn-block login-btn">
                  登录
                </el-button>
              </section>
              <transition name="fade">
                <div class="login-loading"
                     :class="{'show':loading}">
                  <i class="el-icon-loading"></i>
                </div>
              </transition>
            </div>
          </div>
        </div>
        <div class="col-lg-5 col-md-12 col-pad-0 bg-img none-992">
          <div class="information">
            <h3>{{showCopywriting.title}}</h3>
            <p>{{showCopywriting.content}}</p>
          </div>
        </div>
      </div>
    </div>
    <footer>
      <div class="copyright">
        筑栈（上海）信息技术有限公司 KodeRover ©{{moment().format('YYYY')}}
      </div>
    </footer>
  </div>
</template>
<script>
import { mapGetters } from 'vuex';
import { userLoginAPI, getCurrentUserInfoAPI } from '@api';
import moment from 'moment';
import { isMobile } from 'mobile-device-detect';
export default {
  data() {
    return {
      username: '',
      password: '',
      loading: false,
      loginForm: {
        username: '',
        password: '',
      },
      rules: {
        username: [
          { required: true, message: '请输入邮箱', trigger: 'change' }
        ],
        password: [
          { required: true, message: '请输入密码', trigger: 'change' }
        ],
      },
      moment,
      copywriting: {
        common: {
          title: '高效研发从现在开始',
          content: '面向开发者设计的高可用 CI/CD：Zadig 强大的云原生多环境能力，轻松实现本地联调、微服务并行构建、集成测试和持续部署。'
        }
      }
    }
  },
  methods: {
    login() {
      this.$refs['loginForm'].validate((valid) => {
        if (valid) {
          this.loading = true;
          const payload = {
            email: this.loginForm.username,
            password: this.loginForm.password,
          }
          const org_id = 1
          userLoginAPI(org_id, payload).then((res) => {
            this.$store.commit('INJECT_PROFILE', res);
            this.$message({
              message: '登录成功，欢迎 ' + this.$store.state.login.userinfo.info.name,
              type: 'success'
            });
            this.redirectByDevice();

          }, () => {
            this.loading = false;
          })
        } else {
          return false;
        }
      });
    },
    checkLogin() {
      getCurrentUserInfoAPI().then(
        response => {
          this.$store.commit('INJECT_PROFILE', response);
          this.redirectByDevice();
        }
      );

    },
    redirectByDevice() {
      if (isMobile) {
        this.$router.push('/mobile');
      }
      else {
        if (typeof this.$route.query.redirect !== 'undefined' && this.$route.query.redirect !== '/') {
          this.$router.push(this.$route.query.redirect);
        } else {
          this.$router.push('/v1/projects');
        }
      }
    }
  },
  computed: {
    ...mapGetters([
      'signupStatus',
    ]),
    showCopywriting() {
      return this.copywriting.common;
    },
  },

  mounted() {
    this.checkLogin();
  },
}
</script>
<style lang="less" scoped>
@import url("~@assets/css/common/bootstarp.less");
.login .login-inner-form .col-pad-0 {
  padding: 0;
}
.login-btn {
  margin-bottom: 18px;
}
.login .bg-img {
  background: rgba(0, 0, 0, 0.04) url("~@assets/background/login.jpg") top left
    repeat;
  background-size: cover;
  top: 0;
  width: 100%;
  bottom: 0;
  min-height: 100vh;
  text-align: left;
  z-index: 999;
  opacity: 1;
  border-radius: 100% 0 0 100%;
  position: relative;
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 30px 30px;
}

.login .form-section {
  min-height: 100vh;
  position: relative;
  text-align: center;
  display: -webkit-box;
  display: -moz-box;
  display: -ms-flexbox;
  display: -webkit-flex;
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 30px;
}

.login .login-inner-form {
  max-width: 350px;
  color: #717171;
  width: 100%;
  text-align: center;
}

.login .login-inner-form p {
  color: #717171;
  font-size: 14px;
  margin: 0;
}

.login .login-inner-form p a {
  color: #717171;
  font-weight: 500;
}

.login .login-inner-form img {
  margin-bottom: 15px;
  height: 60px;
}

.login .login-inner-form h3 {
  margin: 0 0 25px;
  font-size: 14px;
  font-weight: 400;
  color: #717171;
}

.login .login-inner-form .form-group {
  margin-bottom: 25px;
}

.login .login-inner-form .input-text {
  outline: none;
  width: 100%;
  padding: 10px 20px;
  font-size: 13px;
  outline: 0;
  font-weight: 600;
  color: #717171;
  height: 45px;
  border-radius: 50px;
  border: 1px solid #dbdbdb;
  box-shadow: 0 1px 3px 0 rgba(0, 0, 0, 0.06);
}

.login .login-inner-form .btn-md {
  cursor: pointer;
  padding: 12px 30px 11px 30px;
  letter-spacing: 1px;
  font-size: 14px;
  font-weight: 600;
  border-radius: 50px;
}

.login .login-inner-form input[type="checkbox"],
input[type="radio"] {
  margin-right: 3px;
}

.login .login-inner-form button:focus {
  outline: none;
  outline: 0 auto -webkit-focus-ring-color;
}

.login .login-inner-form .btn-theme.focus,
.btn-theme:focus {
  box-shadow: none;
}

.login .login-inner-form .btn-theme {
  background: #376bff;
  border: none;
  color: #fff;
}

.login .login-inner-form .btn-theme:hover {
  background: #2c5ce4;
}

.login .login-inner-form .checkbox .terms {
  margin-left: 3px;
}

.login .information {
  color: #fff;
  margin: 0 20px 0 70px;
}

.login .information h3 {
  color: #fff;
  margin-bottom: 20px;
  font-size: 25px;
}

.login .information p {
  color: #fff;
  margin-bottom: 20px;
  line-height: 28px;
}

.login .social-box .social-list {
  margin: 0;
  padding: 0;
  list-style: none;
}

.login .social-box .social-list li {
  font-size: 16px;
  float: left;
}

.login .social-box .social-list li a {
  margin-right: 20px;
  font-size: 25px;
  border-radius: 3px;
  display: inline-block;
  color: #fff;
}

.login .none-2 {
  display: none;
}

.login .btn-section {
  margin-bottom: 30px;
}

.login .information .link-btn:hover {
  text-decoration: none;
}

.login .information .link-btn {
  background: #fff;
  padding: 6px 30px;
  font-size: 13px;
  border-radius: 3px;
  margin: 3px;
  letter-spacing: 1px;
  font-weight: 600;
  color: #376bff;
}

.login .information .active {
  background: #376bff;
  color: #fff;
}

.login .login-inner-form .terms {
  margin-left: 3px;
}

.login .login-inner-form .checkbox {
  margin-bottom: 25px;
  font-size: 14px;
}

.login .login-inner-form .form-check {
  float: left;
  margin-bottom: 0;
}

.login .login-inner-form .form-check a {
  color: #717171;
  float: right;
}

.login .login-inner-form .form-check-input {
  position: absolute;
  margin-left: 0;
}

.login .login-inner-form .form-check label::before {
  content: "";
  display: inline-block;
  position: absolute;
  width: 17px;
  height: 17px;
  margin-left: -25px;
  border: 1px solid #cccccc;
  border-radius: 3px;
  background-color: #fff;
}

.login .login-inner-form .form-check-label {
  padding-left: 25px;
  margin-bottom: 0;
  font-size: 14px;
}

.login
  .login-inner-form
  .checkbox-theme
  input[type="checkbox"]:checked
  + label::before {
  background-color: #376bff;
  border-color: #376bff;
}

.login .login-inner-form input[type="checkbox"]:checked + label:before {
  font-weight: 300;
  color: #f3f3f3;
  line-height: 15px;
  font-size: 14px;
  content: "\2713";
}

.login .login-inner-form input[type="checkbox"],
input[type="radio"] {
  margin-top: 4px;
}

.login .login-inner-form .checkbox a {
  font-size: 14px;
  color: #717171;
  float: right;
}

.login .facebook-color:hover {
  color: #4867aa !important;
}

.login .twitter-color:hover {
  color: #33ccff !important;
}

.login .google-color:hover {
  color: #db4437 !important;
}

.login .linkedin-color:hover {
  color: #0177b5 !important;
}

@media (max-width: 992px) {
  .login .none-992 {
    display: none;
  }

  .login .none-2 {
    display: block;
  }

  .login .form-section {
    padding: 30px 15px;
  }
}
.login-loading {
  width: 30px;
  margin: 0 auto;
  font-size: 30px;
  margin-top: 12px;
  visibility: hidden;
  &.show {
    visibility: unset;
  }
}
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.5s;
}
.fade-enter,
.fade-leave-active {
  opacity: 0;
}

.details {
  font-size: 15px;
  /deep/ .el-input {
    .el-input__inner {
      border-radius: 20px;
    }
  }
  /deep/ .el-form-item__label {
    color: #c0c4cc;
  }
  .name {
    font-size: 35px;
    display: inline-block;
    margin-bottom: 5px;
    font-weight: 500;
    font-family: "Helvetica Neue";
    background: linear-gradient(25deg, #ff6302 0%, #ff2968 100%);
    display: inline-block;
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
  }
  .github-details {
    margin: 10px auto 0px;
    padding: 20px;
    a {
      user-select: none;
      display: inline-block;
      width: 100%;
      height: 45px;
      color: white;
      font-weight: 400;
      font-size: 18px;
      transition: all 0.3s;
      &:hover {
        text-decoration: none;
      }
    }
    p {
      margin: 10px auto 25px;
      color: #524f4f;
      font-size: 15px;
      text-align: left;
    }
    .login-button-show {
      display: block;
      border-radius: 5px;
      background: #376bff;
      line-height: 45px;
      letter-spacing: 1px;
      &:hover {
        background: #2c5ce4;
      }
      .iconfont {
        font-size: 26px;
        position: relative;
        top: 2px;
        margin-right: 3px;
      }
    }
  }
}

footer {
  display: flex;
  justify-content: center;
  .copyright {
    margin-bottom: 30px;
    color: #8f9bb2;
    position: absolute;
    bottom: 0px;
    font-size: 14px;
  }
}
</style>