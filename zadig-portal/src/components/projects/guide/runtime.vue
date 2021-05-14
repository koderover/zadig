<template>
  <div class="projects-runtime-container">
    <div class="guide-container">
      <step :activeStep="2">
      </step>
      <div class="current-step-container">
        <div class="title-container">
          <span class="first">第三步</span>
          <span class="second">将服务加入运行环境，并准备对应的交付工作流，后续均可在项目中进行配置</span>
        </div>
        <div class="account-integrations cf-block__list">
          <div class="title">
            <h4>环境准备</h4>
            <el-alert v-if="envFailure.length > 0||timeOut"
                      type="warning">
              <template v-solt:title>
                环境正在准备中，可点击 “下一步” 或者
                <el-button type="text"
                           @click="jumpEnv">查看环境状态</el-button>
                <i v-if="jumpLoading"
                   class="el-icon-loading"></i>
              </template>
            </el-alert>
          </div>
          <div class="cf-block__item">
            <div class="account-box-item">
              <div class="account-box-item__info integration-card">
                <div class="integration-card__image">
                  <el-button v-if="envSuccess.length === 2"
                             type="success"
                             icon="el-icon-check"
                             circle></el-button>
                  <el-button v-else
                             icon="el-icon-loading"
                             circle></el-button>
                </div>
                <div class="integration-card__info">
                  <div v-for="(env,index) in envStatus"
                       :key="index"
                       class="integration-details">
                    <template v-if="env.env_name==='dev'">
                      <span class="env-name">
                        开发环境：{{projectName}}-dev
                      </span>
                      <span class="desc">，开发日常自测、业务联调</span>
                      <el-link v-if="env.err_message!==''"
                               type="warning">{{env.err_message}}</el-link>
                    </template>
                    <template v-if="env.env_name==='qa'"
                              class="env-item">
                      <span class="env-name">测试环境：{{projectName}}-qa
                      </span>
                      <span class="desc">，测试环境（自动化测试、业务验收）</span>
                      <el-link v-if="env.err_message!==''"
                               type="warning">{{env.err_message}}</el-link>
                    </template>

                  </div>
                </div>
              </div>
              <div class="account-box-item__controls">
              </div>
            </div>
          </div>
          <div class="title">
            <h4>工作流准备</h4>
            <el-alert v-if="pipeStatus.err_message"
                      :title="pipeStatus.err_message"
                      type="error">
            </el-alert>
          </div>
          <div class="cf-block__item">
            <div class="account-box-item">
              <div class="account-box-item__info integration-card">
                <div class="integration-card__image">
                  <el-button v-if="pipeStatus.status === 'success'"
                             type="success"
                             icon="el-icon-check"
                             circle></el-button>
                  <el-button v-else
                             icon="el-icon-loading"
                             circle></el-button>
                </div>
                <div class="integration-card__info">
                  <div class="integration-details">
                    开发工作流：{{projectName}}-workflow-dev，应用日常升级，用于开发自测和联调
                  </div>
                  <div class="integration-details">
                    测试工作流：{{projectName}}-workflow-qa，应用按需升级，用于自动化测试和业务验收
                  </div>
                  <div class="integration-details">
                    运维工作流：{{projectName}}-workflow-ops，业务按需发布，用于版本升级和业务上线
                  </div>
                </div>
              </div>
              <div class="account-box-item__controls">
              </div>
            </div>
          </div>

        </div>
      </div>
    </div>
    <div class="controls__wrap">
      <div class="controls__right">
        <router-link :to="`/v1/projects/create/${projectName}/basic/delivery`">
          <button v-if="!getResult"
                  type="primary"
                  class="save-btn"
                  disabled
                  plain>下一步</button>
          <button v-else-if="getResult"
                  type="primary"
                  class="save-btn"
                  plain>下一步</button>
        </router-link>
        <div class="run-button">
        </div>
      </div>
    </div>
  </div>
</template>
<script>
import bus from '@utils/event_bus';
import step from './common/step.vue';
import { generateEnvAPI, generatePipeAPI } from '@api';
export default {
  data() {
    return {
      envStatus: [{ env_name: 'dev' }, { env_name: 'qa' }],
      pipeStatus: {},
      getResult: false,
      envTimer: 0,
      pipeTimer: 0,
      secondCount: 0,
      timeOut: 0,
      jumpLoading: false,
    }
  },
  methods: {
    jumpEnv() {
      this.$confirm('确认跳出后就不再进入 onboarding 流程。', '确认跳出产品交付向导？', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }).then(() => {
        this.jumpLoading = true;
        this.saveOnboardingStatus(this.projectName, 0).then((res) => {
          this.$router.push(`/v1/projects/detail/${this.projectName}/envs`);
        }).catch(() => {
          this.jumpLoading = false;
        })
      }).catch(() => {
        this.$message.info('取消跳转');
      })
    },
    generateEnv(projectName, envType) {
      const getEnv = new Promise((resolve, reject) => {
        generateEnvAPI(projectName, envType).then((res) => {
          this.$set(this, 'envStatus', res);
          resolve(res);
        }).catch((err) => {
          console.log(err)
        })
      });
      getEnv.then((env_res) => {
        const successResult = env_res.filter(env => env.status === 'Running');
        const failureResult = env_res.filter(env => (env.err_message && env.err_message !== ''));
        if (successResult.length === 2) {
          clearInterval(this.envTimer);
          this.timeOut = false;
          this.getResult = true;
        }
        if (failureResult.length >= 1) {
          clearInterval(this.envTimer);
          this.timeOut = false;
          this.getResult = true;
        }
        if (this.secondCount === 60) {
          clearInterval(this.envTimer);
          this.timeOut = true;
          this.getResult = true;
        }
        this.pipeTimer = setInterval(() => {
          this.generatePipe(this.projectName);
        }, 1000);

      })

    },
    generatePipe(projectName) {
      if (this.pipeStatus.status === 'success') {
        clearInterval(this.pipeTimer);
      } else {
        generatePipeAPI(projectName).then((res) => {
          this.$set(this, 'pipeStatus', res);
        })
      }
    },
  },
  computed: {
    projectName() {
      return this.$route.params.project_name;
    },
    envSuccess() {
      const result = this.envStatus.filter(env => env.status === 'Running');
      return result;
    },
    envType() {
      return this.$route.query.serviceType;
    },
    envFailure() {
      const result = this.envStatus.filter(env => (env.err_message && env.err_message !== ''));
      return result;
    }
  },
  watch: {
    secondCount: function (new_val, old_val) {
      if (new_val === 60 && this.envSuccess.length < 2) {
      }
    }
  },
  created() {
    if (this.envTimer) {
      clearInterval(this.envTimer)
    } else {
      this.envTimer = setInterval(() => {
        this.secondCount++;
        this.generateEnv(this.projectName, this.envType);
      }, 1000)
    };
    bus.$emit(`show-sidebar`, true);
    bus.$emit(`set-topbar-title`, { title: '', breadcrumb: [{ title: '项目', url: '/v1/projects' }, { title: this.projectName, url: '' }] });
    bus.$emit(`set-sub-sidebar-title`, {
      title: '',
      routerList: []
    });
  },
  beforeDestroy() {
    clearInterval(this.envTimer);
    clearInterval(this.pipeTimer);
  },
  components: {
    step
  },
  onboardingStatus: 3
}
</script>

<style lang="less">
.projects-runtime-container {
  flex: 1;
  position: relative;
  overflow: auto;
  background-color: #f5f7f7;

  .page-title-container {
    display: flex;
    padding: 0 20px;
    h1 {
      text-align: center;
      width: 100%;
      color: #4c4c4c;
      font-weight: 300;
    }
  }
  .guide-container {
    margin-top: 10px;
    min-height: calc(~"100% - 70px");
    .current-step-container {
      .title-container {
        margin-left: 20px;
        .first {
          font-size: 18px;
          background: #3289e4;
          color: #fff;
          font-weight: 300;
          padding: 8px;
          display: inline-block;
          width: 110px;
          text-align: center;
        }
        .second {
          font-size: 13px;
          color: #4c4c4c;
        }
      }
      .account-integrations {
        .el-alert--warning {
          .el-button--text {
            color: inherit;
          }
        }
      }
      .cf-block__list {
        -webkit-box-flex: 1;
        -ms-flex: 1;
        flex: 1;
        overflow-y: auto;
        background-color: inherit;
        padding: 0 30px;
        margin-top: 15px;
        .title {
          h4 {
            color: #4c4c4c;
            margin: 10px 0;
            font-weight: 400;
            text-decoration: underline;
          }
          a {
            color: inherit;
            text-decoration-color: inherit;
          }
        }
        .cf-block__item {
          min-height: 102px;
          .account-box-item {
            display: -webkit-box;
            display: -ms-flexbox;
            display: flex;
            -webkit-box-align: center;
            -ms-flex-align: center;
            align-items: center;
            -webkit-box-pack: justify;
            -ms-flex-pack: justify;
            justify-content: space-between;
            margin-bottom: 10px;
            padding: 20px 30px;
            background-color: #fff;
            -webkit-box-shadow: 0 3px 2px 1px rgba(0, 0, 0, 0.05);
            box-shadow: 0 3px 2px 1px rgba(0, 0, 0, 0.05);
            filter: progid:DXImageTransform.Microsoft.dropshadow(OffX=0, OffY=3px, Color='#0D000000');
            .integration-card {
              display: -webkit-box;
              display: -ms-flexbox;
              display: flex;
              -webkit-box-align: center;
              -ms-flex-align: center;
              align-items: center;
              -webkit-box-pack: start;
              -ms-flex-pack: start;
              justify-content: flex-start;
              .integration-card__image {
                width: 64px;
                .el-button.is-circle {
                  border-radius: 50%;
                  padding: 6px;
                }
              }
              .cf-sub-title {
                font-size: 16px;
                font-weight: bold;
                text-align: left;
                color: #2f2f2f;
              }
              .integration-details {
                margin-bottom: 5px;
                color: #4c4c4c;
                line-height: 20px;
                font-size: 14px;
                .env-name {
                  display: inline-block;
                }
                .desc {
                  display: inline-block;
                  width: 250px;
                }
              }
            }
            .integration-card > * {
              -webkit-box-flex: 0;
              -ms-flex: 0 0 auto;
              flex: 0 0 auto;
            }
          }
        }
      }
    }
  }
  .alert {
    display: flex;
    padding: 0 25px;
    .el-alert {
      margin-bottom: 35px;
      .el-alert__title {
        font-size: 15px;
      }
    }
  }
  .controls__wrap {
    position: relative;
    bottom: 0;
    left: 0;
    right: 0;
    height: 60px;
    background-color: #fff;
    padding: 0 10px;
    z-index: 2;
    margin: 0 15px;
    box-shadow: 0 4px 4px 0 rgba(0, 0, 0, 0.05);
    display: flex;
    align-items: center;
    justify-content: space-between;
    > * {
      margin-right: 10px;
    }
    .controls__right {
      display: -webkit-box;
      display: -ms-flexbox;
      display: flex;
      -webkit-box-align: center;
      -ms-flex-align: center;
      align-items: center;
      .save-btn {
        text-decoration: none;
        background-color: #1989fa;
        color: #fff;
        padding: 10px 17px;
        border: 1px solid #1989fa;
        font-size: 13px;
        font-weight: bold;
        transition: background-color 300ms, color 300ms, border 300ms;
        cursor: pointer;
        margin-right: 15px;
      }
      .save-btn[disabled] {
        background-color: #9ac9f9;
        border: 1px solid #9ac9f9;
        cursor: not-allowed;
      }
    }
  }
}
</style>