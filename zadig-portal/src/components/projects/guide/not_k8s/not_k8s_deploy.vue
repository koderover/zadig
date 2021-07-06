<template>
  <div class="projects-not-k8s-deploy-container">
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
                        开发环境：dev
                      </span>
                      <span class="desc">，开发日常自测、业务联调</span>
                      <el-link v-if="env.err_message!==''"
                               type="warning">{{env.err_message}}</el-link>
                    </template>
                    <template v-if="env.env_name==='qa'"
                              class="env-item">
                      <span class="env-name">测试环境：qa
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
        <router-link :to="`/v1/projects/create/${projectName}/not_k8s/delivery`">
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
import bus from '@utils/event_bus'
import step from './container/step_not_k8s.vue'
import { generateEnvAPI, generatePipeAPI } from '@api'
export default {
  data () {
    return {
      envStatus: [{ env_name: 'dev' }, { env_name: 'qa' }],
      pipeStatus: {},
      getResult: false,
      envTimer: 0,
      pipeTimer: 0,
      secondCount: 0,
      timeOut: 0
    }
  },
  methods: {
    jumpEnv () {
      this.$confirm('确认跳出后就不再进入 onboarding 流程。', '确认跳出产品交付向导？', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        this.jumpLoading = true
        this.saveOnboardingStatus(this.projectName, 0).then((res) => {
          this.$router.push(`/v1/projects/detail/${this.projectName}/envs`)
        }).catch(() => {
          this.jumpLoading = false
        })
      }).catch(() => {
        this.$message.info('取消跳转')
      })
    },
    generateEnv (projectName) {
      const getEnv = new Promise((resolve, reject) => {
        generateEnvAPI(projectName).then((res) => {
          this.$set(this, 'envStatus', res)
          resolve(res)
        }).catch((err) => {
          console.log(err)
        })
      })
      getEnv.then((env_res) => {
        const successResult = env_res.filter(env => env.status === 'Running')
        const failureResult = env_res.filter(env => (env.err_message && env.err_message !== ''))
        if (successResult.length === 2) {
          clearInterval(this.envTimer)
          this.timeOut = false
          this.getResult = true
        }
        if (failureResult.length >= 1) {
          clearInterval(this.envTimer)
          this.timeOut = false
          this.getResult = true
        }
        if (this.secondCount === 60) {
          clearInterval(this.envTimer)
          this.timeOut = true
          this.getResult = true
        }
        this.pipeTimer = setInterval(() => {
          this.generatePipe(this.projectName)
        }, 1000)
      })
    },
    generatePipe (projectName) {
      if (this.pipeStatus.status === 'success') {
        clearInterval(this.pipeTimer)
      } else {
        generatePipeAPI(projectName).then((res) => {
          this.$set(this, 'pipeStatus', res)
        })
      }
    }
  },
  computed: {
    projectName () {
      return this.$route.params.project_name
    },
    envSuccess () {
      const result = this.envStatus.filter(env => env.status === 'Running')
      return result
    },
    envFailure () {
      const result = this.envStatus.filter(env => (env.err_message && env.err_message !== ''))
      return result
    }
  },
  created () {
    if (this.envTimer) {
      clearInterval(this.envTimer)
    } else {
      this.envTimer = setInterval(() => {
        this.secondCount++
        this.generateEnv(this.projectName)
      }, 1000)
    };
    bus.$emit(`show-sidebar`, true)
    bus.$emit(`set-topbar-title`, { title: '', breadcrumb: [{ title: '项目', url: '/v1/projects' }, { title: this.projectName, url: '' }] })
    bus.$emit(`set-sub-sidebar-title`, {
      title: '',
      routerList: []
    })
  },
  beforeDestroy () {
    clearInterval(this.envTimer)
    clearInterval(this.pipeTimer)
  },
  components: {
    step
  },
  onboardingStatus: 3
}
</script>

<style lang="less">
.projects-not-k8s-deploy-container {
  position: relative;
  flex: 1;
  overflow: auto;
  background-color: #f5f7f7;

  .page-title-container {
    display: flex;
    padding: 0 20px;

    h1 {
      width: 100%;
      color: #4c4c4c;
      font-weight: 300;
      text-align: center;
    }
  }

  .guide-container {
    min-height: calc(~"100% - 70px");
    margin-top: 10px;

    .current-step-container {
      .title-container {
        margin-left: 20px;

        .first {
          display: inline-block;
          width: 110px;
          padding: 8px;
          color: #fff;
          font-weight: 300;
          font-size: 18px;
          text-align: center;
          background: #3289e4;
        }

        .second {
          color: #4c4c4c;
          font-size: 13px;
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
        -ms-flex: 1;
        flex: 1;
        margin-top: 15px;
        padding: 0 30px;
        overflow-y: auto;
        background-color: inherit;
        -webkit-box-flex: 1;

        .title {
          h4 {
            margin: 10px 0;
            color: #4c4c4c;
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
            align-items: center;
            justify-content: space-between;
            margin-bottom: 10px;
            padding: 20px 30px;
            background-color: #fff;
            -webkit-box-shadow: 0 3px 2px 1px rgba(0, 0, 0, 0.05);
            box-shadow: 0 3px 2px 1px rgba(0, 0, 0, 0.05);
            filter: progid:dximagetransform.microsoft.dropshadow(OffX=0, OffY=3px, Color='#0D000000');
            -webkit-box-align: center;
            -ms-flex-align: center;
            -webkit-box-pack: justify;
            -ms-flex-pack: justify;

            .integration-card {
              display: -webkit-box;
              display: -ms-flexbox;
              display: flex;
              align-items: center;
              justify-content: flex-start;
              -webkit-box-align: center;
              -ms-flex-align: center;
              -webkit-box-pack: start;
              -ms-flex-pack: start;

              .integration-card__image {
                width: 64px;

                .el-button.is-circle {
                  padding: 6px;
                  border-radius: 50%;
                }
              }

              .cf-sub-title {
                color: #2f2f2f;
                font-weight: bold;
                font-size: 16px;
                text-align: left;
              }

              .integration-details {
                margin-bottom: 5px;
                color: #4c4c4c;
                font-size: 14px;
                line-height: 20px;

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
              -ms-flex: 0 0 auto;
              flex: 0 0 auto;
              -webkit-box-flex: 0;
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
    right: 0;
    bottom: 0;
    left: 0;
    z-index: 2;
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 60px;
    margin: 0 15px;
    padding: 0 10px;
    background-color: #fff;
    box-shadow: 0 4px 4px 0 rgba(0, 0, 0, 0.05);

    .controls__right {
      display: -webkit-box;
      display: -ms-flexbox;
      display: flex;
      align-items: center;
      margin-right: 10px;
      -webkit-box-align: center;
      -ms-flex-align: center;

      .save-btn {
        margin-right: 15px;
        padding: 10px 17px;
        color: #fff;
        font-weight: bold;
        font-size: 13px;
        text-decoration: none;
        background-color: #1989fa;
        border: 1px solid #1989fa;
        cursor: pointer;
        transition: background-color 300ms, color 300ms, border 300ms;
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
